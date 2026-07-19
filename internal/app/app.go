package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/httpapi"
	"github.com/dwdmsh/school-mdm/internal/httpserver"
	"github.com/dwdmsh/school-mdm/internal/mdm"
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

	stub := &mdm.StubEnqueuer{}
	svc := &approvals.Service{
		Store:     st,
		Enqueue:   stub,
		PortalURL: cfg.PortalBaseURL,
	}
	catalog := &appmeta.Catalog{
		Store:   st,
		Country: cfg.ItunesCountry,
		Lang:    cfg.ItunesLang,
	}

	return &App{
		Cfg:     cfg,
		Log:     log,
		Store:   st,
		Stub:    stub,
		Service: svc,
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
	srv := httpserver.New(a.Cfg.HTTPAddr, a.Handler(), a.Log)
	return srv.ListenAndServe(ctx)
}
