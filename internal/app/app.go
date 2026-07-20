package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/credits"
	"github.com/dwdmsh/school-mdm/internal/httpapi"
	"github.com/dwdmsh/school-mdm/internal/httpserver"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/nedarim"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
	"github.com/dwdmsh/school-mdm/internal/store/postgres"
)

// App is the composition root.
type App struct {
	Cfg     config.Config
	Log     *slog.Logger
	Store   store.Store
	Stub    *mdm.StubEnqueuer
	Service *approvals.Service
	Credits *credits.Service
	Catalog *appmeta.Catalog
	closer  func()
}

// New builds the application from environment config.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var (
		st     store.Store
		closer func()
	)
	if cfg.DatabaseURL != "" {
		pg, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		st = pg
		closer = pg.Close
		log.Info("using postgres store")
	} else {
		st = memory.New()
		closer = func() {}
		log.Info("using memory store (set DATABASE_URL for Neon)")
	}

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

	stub := &mdm.StubEnqueuer{}
	svc := &approvals.Service{
		Store:     st,
		Enqueue:   stub,
		PortalURL: cfg.PortalBaseURL,
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
		Cfg:     cfg,
		Log:     log,
		Store:   st,
		Stub:    stub,
		Service: svc,
		Credits: creditSvc,
		Catalog: catalog,
		closer:  closer,
	}, nil
}

// Close releases resources.
func (a *App) Close() {
	if a.closer != nil {
		a.closer()
	}
}

// Handler returns the HTTP handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	api := &httpapi.API{
		Cfg:     a.Cfg,
		Service: a.Service,
		Credits: a.Credits,
		Catalog: a.Catalog,
		Store:   a.Store,
		Stub:    a.Stub,
		Log:     a.Log,
	}
	api.Mount(mux)
	return mux
}

// Run listens until ctx is done.
func (a *App) Run(ctx context.Context) error {
	go a.Credits.StartAllotmentTicker(ctx, 2*time.Minute, a.Log)
	srv := httpserver.New(a.Cfg.HTTPAddr, a.Handler(), a.Log)
	return srv.ListenAndServe(ctx)
}
