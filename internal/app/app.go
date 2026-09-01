package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/saphka/link-shortener/internal/api"
	"github.com/saphka/link-shortener/internal/config"
	"github.com/saphka/link-shortener/internal/database"
	"github.com/saphka/link-shortener/internal/repository/link"
)

type app struct {
	Name     string
	cfg      config.Config
	pool     *pgxpool.Pool
	mux      *http.ServeMux
	listener net.Listener
	server   *http.Server
}

func NewApp(ctx context.Context, name string, cfg config.Config) (*app, error) {
	promReg := prometheus.NewRegistry()
	promReg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create connection pool: %w", err)
	}
	database.RegisterPoolMetrics(pool, promReg)

	linkRepo := link.NewLinkRepo(pool)
	mux := http.NewServeMux()
	api.SetupMetricsEndpoint(mux, promReg)
	handler, err := api.NewServer(mux, promReg, linkRepo)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot create handler: %w", err)
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot listen on port %d: %w", cfg.Port, err)
	}

	app := &app{
		Name:     name,
		cfg:      cfg,
		mux:      mux,
		pool:     pool,
		listener: listener,
		server: &http.Server{
			Handler:           handler,
			ReadTimeout:       5 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       10 * time.Second,
		},
	}
	return app, nil
}

func (a *app) Run(ctx context.Context) {
	go func() {
		if err := a.server.Serve(a.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "server start failed.", slog.Any("error", err))
		}
	}()

	slog.Info("application running")
	<-ctx.Done()
	ctx, cancel := context.WithTimeout( //nolint:contextcheck
		context.Background(),
		a.cfg.ShutdownPeriod,
	)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "server shutdown failed.", slog.Any("error", err))
	}
	a.pool.Close()

	slog.Info("server exited properly")
}
