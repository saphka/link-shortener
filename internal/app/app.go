package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saphka/link-shortener/internal/api"
	"github.com/saphka/link-shortener/internal/config"
	"github.com/saphka/link-shortener/internal/repository/link"
)

type app struct {
	Name   string
	ctx    context.Context
	cfg    config.Config
	pool   *pgxpool.Pool
	mux    *http.ServeMux
	server *http.Server
}

func NewApp(ctx context.Context, name string, cfg config.Config) (*app, error) {
	pool, err := newPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create connection pool: %w", err)
	}

	linkRepo, err := link.NewLinkRepo(pool)
	if err != nil {
		return nil, fmt.Errorf("cannot create link repo: %w", err)
	}
	mux := http.NewServeMux()
	handler, err := api.NewServer(mux, linkRepo)
	if err != nil {
		return nil, fmt.Errorf("canno create handler: %w", err)
	}

	app := &app{
		Name: name,
		ctx:  ctx,
		cfg:  cfg,
		mux:  mux,
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Port),
			Handler:           handler,
			ReadTimeout:       5 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
	return app, nil
}

func (a *app) Run() {
	go func() {
		if err := a.server.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(a.ctx, "server start failed.", slog.Any("error", err))
		}
	}()

	slog.Info("application running")
	<-a.ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownPeriod)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "server shutdown failed.", slog.Any("error", err))
	}
	a.pool.Close()

	slog.Info("server exited properly")
}

func newPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(
		ctx,
		fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s",
			cfg.DB.User,
			cfg.DB.Password,
			cfg.DB.Host,
			cfg.DB.Port,
			cfg.DB.Name,
		),
	)
	if err != nil {
		return nil, err
	}
	return pool, nil
}
