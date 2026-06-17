package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}


func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	zap.L().Info("Connecting to database...")

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 10 * time.Second

	// Compose slow-query + OTel tracers via pgx's multi-tracer mechanism.
	// otelpgx creates a span per query; slowQueryTracer emits a structured
	// log warning when duration exceeds the threshold. Both run on every
	// query at near-zero overhead.
	config.ConnConfig.Tracer = &multiTracer{
		tracers: []pgx.QueryTracer{
			otelpgx.NewTracer(),
			&slowQueryTracer{threshold: 100 * time.Millisecond},
		},
	}
	_ = tracelog.LogLevelDebug // silence unused import on some builds

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	zap.L().Info("Database connection established")
	return pool, nil
}

// multiTracer fan-outs pgx tracing events to every child tracer. Pgx's
// QueryTracer interface only accepts a single value, so we wrap them.
type multiTracer struct {
	tracers []pgx.QueryTracer
}

func (m *multiTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	for _, t := range m.tracers {
		ctx = t.TraceQueryStart(ctx, conn, data)
	}
	return ctx
}

func (m *multiTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	for _, t := range m.tracers {
		t.TraceQueryEnd(ctx, conn, data)
	}
}

// slowQueryTracer implements pgx.QueryTracer. It records the start time
// inside the ctx via a custom key, then on QueryEnd computes duration and
// warns if it crosses the threshold. Errors are also surfaced regardless
// of duration so failing queries don't hide in the info-level stream.
type slowQueryTracer struct {
	threshold time.Duration
}

type slowQueryStartKey struct{}

func (t *slowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, slowQueryStartKey{}, queryStartCtx{
		sql:   data.SQL,
		args:  data.Args,
		begin: time.Now(),
	})
}

func (t *slowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(slowQueryStartKey{}).(queryStartCtx)
	if !ok {
		return
	}
	dur := time.Since(start.begin)
	switch {
	case data.Err != nil:
		zap.L().Warn("pg query failed",
			zap.String("sql", truncateSQL(start.sql, 256)),
			zap.Duration("duration", dur),
			zap.Int("args", len(start.args)),
			zap.Error(data.Err))
	case dur > t.threshold:
		zap.L().Warn("pg slow query",
			zap.String("sql", truncateSQL(start.sql, 256)),
			zap.Duration("duration", dur),
			zap.Int("args", len(start.args)))
	}
}

type queryStartCtx struct {
	sql   string
	args  []any
	begin time.Time
}

// truncateSQL clips long queries for log readability. Multi-line SQL
// statements are collapsed to a single line.
func truncateSQL(s string, max int) string {
	out := make([]rune, 0, len(s))
	prevSpace := false
	for _, r := range s {
		if r == '\n' || r == '\t' {
			r = ' '
		}
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		out = append(out, r)
		if len(out) >= max {
			out = append(out, '…')
			break
		}
	}
	return string(out)
}
