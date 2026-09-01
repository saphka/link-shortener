package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/saphka/link-shortener/internal/config"
)

func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
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
	err = pool.Ping(ctx)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func RegisterPoolMetrics(pool *pgxpool.Pool, reg prometheus.Registerer) {
	coll := &statsCollector{
		getStats: pool.Stat,

		acquiredConnsDesc: prometheus.NewDesc(
			"pgxpool_acquired_conns",
			"Number of currently acquired connections in the pool.",
			nil, nil),
		idleConnsDesc: prometheus.NewDesc(
			"pgxpool_idle_conns",
			"Number of currently idle conns in the pool.",
			nil, nil),
		maxConnsDesc: prometheus.NewDesc(
			"pgxpool_max_conns",
			"Maximum size of the pool.",
			nil, nil),
	}
	reg.MustRegister(coll)
}

type statsFunc func() *pgxpool.Stat

type statsCollector struct {
	getStats statsFunc

	acquiredConnsDesc *prometheus.Desc
	idleConnsDesc     *prometheus.Desc
	maxConnsDesc      *prometheus.Desc
}

// Collect implements [prometheus.Collector].
func (s *statsCollector) Collect(metrics chan<- prometheus.Metric) {
	stats := s.getStats()

	metrics <- prometheus.MustNewConstMetric(
		s.acquiredConnsDesc,
		prometheus.GaugeValue,
		float64(stats.AcquiredConns()),
	)
	metrics <- prometheus.MustNewConstMetric(
		s.idleConnsDesc,
		prometheus.GaugeValue,
		float64(stats.IdleConns()),
	)
	metrics <- prometheus.MustNewConstMetric(
		s.maxConnsDesc,
		prometheus.GaugeValue,
		float64(stats.MaxConns()),
	)
}

// Describe implements [prometheus.Collector].
func (s *statsCollector) Describe(metrics chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(s, metrics)
}
