package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/abm"
	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/credits"
	"github.com/dwdmsh/school-mdm/internal/devicepush"
	"github.com/dwdmsh/school-mdm/internal/httpapi"
	"github.com/dwdmsh/school-mdm/internal/httpserver"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	mdmenqueue "github.com/dwdmsh/school-mdm/internal/mdm/enqueue"
	"github.com/dwdmsh/school-mdm/internal/mdmdep"
	"github.com/dwdmsh/school-mdm/internal/mdmhub"
	"github.com/dwdmsh/school-mdm/internal/mdmstore"
	mdmstorepg "github.com/dwdmsh/school-mdm/internal/mdmstore/postgres"
	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/nedarim"
	"github.com/dwdmsh/school-mdm/internal/notify"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
	"github.com/dwdmsh/school-mdm/internal/store/postgres"
	"github.com/dwdmsh/school-mdm/internal/webhooks"
)

// App is the composition root.
type App struct {
	Cfg      config.Config
	Log      *slog.Logger
	Store    store.Store
	MDMStore mdmstore.Store
	Hub      *mdmhub.Hub
	Stub     *mdm.StubEnqueuer
	Enqueue  mdm.CommandEnqueuer
	Push     *devicepush.Service
	Service  *approvals.Service
	Credits  *credits.Service
	Catalog  *appmeta.Catalog
	ABM      *abm.Service
	DepHTTP  http.Handler
	Activity *activity.Logger
	Webhooks *webhooks.Service
	closers  []func()
}

// New builds the application from environment config.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var (
		st      store.Store
		closers []func()
	)
	if cfg.DatabaseURL != "" {
		pg, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		st = pg
		closers = append(closers, pg.Close)
		log.Info("using postgres store")
	} else {
		st = memory.New()
		log.Info("using memory store (set DATABASE_URL for Neon)")
	}

	hookSvc := webhooks.New(st, log)
	actLog := &activity.Logger{Store: st, Slog: log, Webhooks: hookSvc}

	nedarimClient := &nedarim.Client{Cfg: nedarim.Config{
		Mode:        cfg.NedarimMode,
		MosadID:     cfg.NedarimMosadID,
		ApiPassword: cfg.NedarimApiPassword,
		ApiValid:    cfg.NedarimApiValid,
		PortalBase:  cfg.PortalBaseURL,
	}}
	creditSvc := &credits.Service{
		Store:       st,
		Nedarim:     nedarimClient,
		AccessCost:  cfg.CreditsAccessCost,
		PortalBase:  cfg.PortalBaseURL,
		WebhookPath: "/api/webhooks/nedarim",
	}
	if err := creditSvc.EnsureSettings(ctx); err != nil {
		return nil, fmt.Errorf("ensure credit settings: %w", err)
	}
	if err := ensureMDMSettings(ctx, st, cfg.MDMDepName); err != nil {
		return nil, fmt.Errorf("ensure mdm settings: %w", err)
	}

	stub := &mdm.StubEnqueuer{}
	var (
		enqueuer mdm.CommandEnqueuer = stub
		hub      *mdmhub.Hub
		mdmSt    mdmstore.Store
	)

	var (
		abmSvc  *abm.Service
		depHTTP http.Handler
	)
	// Device-facing portal/webclip must be a public URL (not localhost).
	devicePortalBase := strings.TrimSpace(cfg.PortalBaseURL)
	if pub := strings.TrimSpace(cfg.MDMPublicURL); pub != "" {
		if devicePortalBase == "" ||
			strings.Contains(devicePortalBase, "localhost") ||
			strings.Contains(devicePortalBase, "127.0.0.1") {
			devicePortalBase = pub
		}
	}

	var pushSvc *devicepush.Service
	if cfg.MDMLive() {
		ms, db, err := mdmstorepg.OpenDSN(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("open mdm store: %w", err)
		}
		closers = append(closers, func() { _ = ms.Close() })
		mdmSt = ms
		hub, err = mdmhub.New(db, mdmhub.Config{
			PublicURL:     cfg.MDMPublicURL,
			MDMTopic:      cfg.MDMTopic,
			Checkin:       cfg.MDMCheckin,
			CertHeader:    cfg.MDMCertHeader,
			SCEPPass:      cfg.MDMSCEPPass,
			SCEPChallenge: cfg.MDMSCEPChallenge,
			Debug:         cfg.MDMDebug,
			OnTokenUpdate: func(ctx context.Context, enrollmentID string) {
				actLog.Log(ctx, activity.Event{
					Category:     store.ActivityCategoryDevices,
					Action:       "enroll",
					ActorType:    store.ActivityActorDevice,
					Actor:        enrollmentID,
					EnrollmentID: enrollmentID,
					Result:       store.ActivityResultInfo,
					Summary:      "מכשיר עודכן / נרשם בניהול",
				})
				if pushSvc == nil {
					return
				}
				if err := pushSvc.Reconcile(ctx, enrollmentID); err != nil {
					log.Warn("device onboard reconcile failed",
						"enrollment_id", enrollmentID,
						"err", err,
					)
					actLog.Log(ctx, activity.Event{
						Category:     store.ActivityCategoryDevices,
						Action:       "enroll_reconcile",
						ActorType:    store.ActivityActorSystem,
						Actor:        "onboard",
						EnrollmentID: enrollmentID,
						Result:       store.ActivityResultError,
						Summary:      "סנכרון לאחר רישום נכשל",
						Detail:       map[string]any{"error": err.Error()},
					})
				}
			},
		})
		if err != nil {
			return nil, fmt.Errorf("mdm hub: %w", err)
		}
		enqueuer = &mdmenqueue.LiveEnqueuer{CE: hub.PushEnq}
		h, store, err := mdmdep.Mount(db, cfg.MDMDebug)
		if err != nil {
			return nil, fmt.Errorf("mdm dep: %w", err)
		}
		depHTTP = h
		abmSvc = abm.NewService(store, &depNameSource{store: st})
		depNameLog := cfg.MDMDepName
		if s, err := st.GetMDMSettings(ctx); err == nil && s.DepName != "" {
			depNameLog = s.DepName
		}
		log.Info("mdm protocol enabled",
			"public_url", cfg.MDMPublicURL,
			"topic", cfg.MDMTopic,
			"checkin", cfg.MDMCheckin,
			"dep_name", depNameLog,
			"device_portal", devicePortalBase,
		)
	} else {
		log.Info("mdm enqueue stub (set MDM_ENQUEUE=live to enable protocol)")
	}

	pushSvc = &devicepush.Service{Store: st, MDMStore: mdmSt, Enqueue: enqueuer, PortalURL: devicePortalBase, Log: log}
	push := pushSvc
	notifySvc := &notify.Service{Store: st, Log: log}
	svc := &approvals.Service{
		Store:     st,
		Enqueue:   enqueuer,
		Push:      push,
		Notify:    notifySvc,
		PortalURL: devicePortalBase,
		Credits:   creditSvc,
	}
	catalog := &appmeta.Catalog{
		Store:   st,
		Log:     log,
		Country: cfg.ItunesCountry,
		Lang:    cfg.ItunesLang,
	}

	accessCost := creditSvc.AccessRequestCost(ctx)
	log.Info("credits configured",
		"nedarim_mode", cfg.NedarimMode,
		"access_cost", accessCost,
	)

	return &App{
		Cfg:      cfg,
		Log:      log,
		Store:    st,
		MDMStore: mdmSt,
		Hub:      hub,
		Stub:     stub,
		Enqueue:  enqueuer,
		Push:     push,
		Service:  svc,
		Credits:  creditSvc,
		Catalog:  catalog,
		ABM:      abmSvc,
		DepHTTP:  depHTTP,
		Activity: actLog,
		Webhooks: hookSvc,
		closers:  closers,
	}, nil
}

// Close releases resources.
func (a *App) Close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		if a.closers[i] != nil {
			a.closers[i]()
		}
	}
}

// Handler returns the HTTP handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mdmhub.MountProtocol(mux, a.Hub)
	api := &httpapi.API{
		Cfg:      a.Cfg,
		Service:  a.Service,
		Credits:  a.Credits,
		Catalog:  a.Catalog,
		Store:    a.Store,
		MDMStore: a.MDMStore,
		Push:     a.Push,
		Enqueue:  a.Enqueue,
		Stub:     a.Stub,
		ABM:      a.ABM,
		Activity: a.Activity,
		Webhooks: a.Webhooks,
		Log:      a.Log,
	}
	if a.Service != nil {
		api.Notify = a.Service.Notify
	}
	if a.DepHTTP != nil {
		mux.Handle("/dep/version", a.DepHTTP)
		mux.Handle("/dep/", api.DepAuth(a.DepHTTP))
	}
	api.Mount(mux)
	return mux
}

// Run listens until ctx is done.
func (a *App) Run(ctx context.Context) error {
	go a.Credits.StartAllotmentTicker(ctx, 2*time.Minute, a.Log)
	if a.Cfg.MDMLive() && a.Push != nil {
		go func() {
			// TODO(school-mdm): Startup reconcile-all is a blunt hammer — it re-pushes
			// allowlist + web clip to every known device a few seconds after boot.
			// That can surprise admins (sudden APNs/NotNow storms, duplicate InstallProfile
			// when nothing changed) and doesn't track last-pushed policy version.
			// Replace later with: push only on enroll/TokenUpdate + policy change, and/or
			// a cheap "needs reconcile" check (profile hash / generation) before enqueue.
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			act := a.Activity
			if act == nil {
				act = &activity.Logger{Store: a.Store, Slog: a.Log}
			}
			if err := a.Push.ReconcileAllDevices(ctx); err != nil {
				a.Log.Warn("startup reconcile-all failed", "err", err)
				act.Log(ctx, activity.Event{
					Category:  store.ActivityCategorySystem,
					Action:    "reconcile_all",
					ActorType: store.ActivityActorSystem,
					Actor:     "startup",
					Result:    store.ActivityResultError,
					Summary:   "סנכרון התחלה נכשל לכל המכשירים",
					Detail:    map[string]any{"error": err.Error()},
				})
				return
			}
			a.Log.Info("startup reconcile-all queued for known devices")
			act.Log(ctx, activity.Event{
				Category:  store.ActivityCategorySystem,
				Action:    "reconcile_all",
				ActorType: store.ActivityActorSystem,
				Actor:     "startup",
				Result:    store.ActivityResultInfo,
				Summary:   "סנכרון התחלה לתור לכל המכשירים הידועים",
			})
		}()
	}
	srv := httpserver.New(a.Cfg.HTTPAddr, a.Handler(), a.Log)
	return srv.ListenAndServe(ctx)
}
