package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const instrScope = "github.com/workshop/tapas-backend/db"
const schemaURL = "https://opentelemetry.io/schemas/1.40.0"

// DB wraps *sql.DB with OpenTelemetry instrumentation following the
// OpenTelemetry database semantic conventions:
// https://opentelemetry.io/docs/specs/semconv/database/database-spans/
type DB struct {
	*sql.DB
	tracer        trace.Tracer
	queryDuration metric.Float64Histogram
	commonAttrs   attribute.Set
}

// Row wraps *sql.Row so the span and duration metric are recorded when
// Scan (or Err) is called, at which point the query result is available.
type Row struct {
	*sql.Row
	span          trace.Span
	ctx           context.Context
	startTime     time.Time
	queryDuration metric.Float64Histogram
	metricAttrs   attribute.Set
	once          sync.Once
}

func (r *Row) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.once.Do(func() {
		elapsed := time.Since(r.startTime).Seconds()
		endSpan(r.ctx, r.span, err)
		r.queryDuration.Record(r.ctx, elapsed, metricOptions(r.metricAttrs, err)...)
	})
	return err
}

func (r *Row) Err() error {
	err := r.Row.Err()
	r.once.Do(func() {
		elapsed := time.Since(r.startTime).Seconds()
		endSpan(r.ctx, r.span, err)
		r.queryDuration.Record(r.ctx, elapsed, metricOptions(r.metricAttrs, err)...)
	})
	return err
}

// newDB wraps sqlDB with OTel instrumentation. dsn is parsed to extract
// server address, port, and database name for span and metric attributes.
func NewDB(sqlDB *sql.DB, dsn string) (*DB, error) {
	tracer := otel.Tracer(instrScope, trace.WithSchemaURL(schemaURL))
	meter := otel.Meter(instrScope, metric.WithSchemaURL(schemaURL))

	dur, err := meter.Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of database client operations."),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.0),
	)
	if err != nil {
		return nil, err
	}

	return &DB{
		DB:            sqlDB,
		tracer:        tracer,
		queryDuration: dur,
		commonAttrs:   parseConnAttrs(dsn),
	}, nil
}

// parseConnAttrs extracts server address, port, and database name from a
// postgres:// DSN and returns them as a set of common span/metric attributes.
func parseConnAttrs(dsn string) attribute.Set {
	attrs := []attribute.KeyValue{semconv.DBSystemNamePostgreSQL}

	u, err := url.Parse(dsn)
	if err != nil {
		return attribute.NewSet(attrs...)
	}
	host, portStr, splitErr := net.SplitHostPort(u.Host)
	if splitErr != nil {
		host = u.Host
	}
	if host != "" {
		attrs = append(attrs, semconv.ServerAddress(host))
	}
	if port, err := strconv.Atoi(portStr); err == nil {
		attrs = append(attrs, semconv.ServerPort(port))
	}
	if dbName := strings.TrimPrefix(u.Path, "/"); dbName != "" {
		attrs = append(attrs, semconv.DBNamespace(dbName))
	}
	return attribute.NewSet(attrs...)
}

// QueryContext executes a query that returns rows. A client span is created for
// the duration of the call; errors are recorded on the span and set its status.
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	spanCtx, span, metricAttrs := db.startSpan(ctx, query)
	start := time.Now()
	rows, err := db.DB.QueryContext(spanCtx, query, args...)
	elapsed := time.Since(start).Seconds()
	endSpan(spanCtx, span, err)
	db.queryDuration.Record(ctx, elapsed, metricOptions(metricAttrs, err)...)
	return rows, err
}

// QueryRowContext executes a query expected to return one row. The span and
// duration metric are recorded when the returned Row's Scan method is called.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *Row {
	spanCtx, span, metricAttrs := db.startSpan(ctx, query)
	start := time.Now()
	row := db.DB.QueryRowContext(spanCtx, query, args...)
	return &Row{
		Row:           row,
		span:          span,
		ctx:           spanCtx,
		startTime:     start,
		queryDuration: db.queryDuration,
		metricAttrs:   metricAttrs,
	}
}

// ExecContext executes a query that doesn't return rows. A client span is
// created for the duration; errors are recorded on the span.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	spanCtx, span, metricAttrs := db.startSpan(ctx, query)
	start := time.Now()
	result, err := db.DB.ExecContext(spanCtx, query, args...)
	elapsed := time.Since(start).Seconds()
	endSpan(spanCtx, span, err)
	db.queryDuration.Record(ctx, elapsed, metricOptions(metricAttrs, err)...)
	return result, err
}

func metricOptions(attrs attribute.Set, err error) []metric.RecordOption {
	opts := []metric.RecordOption{metric.WithAttributeSet(attrs)}
	if err != nil {
		opts = append(opts, metric.WithAttributes(attribute.String("error.type", fmt.Sprintf("%T", err))))
	}
	return opts
}

// startSpan creates a client span with database semantic convention attributes
// derived from the query and the connection's common attributes.
//
// Span name and db.query.summary are set to a low-cardinality summary of the
// query (e.g. "SELECT restaurants") per the OTel database naming convention:
// https://opentelemetry.io/docs/specs/semconv/database/database-spans/#generating-a-summary-of-the-query
func (db *DB) startSpan(ctx context.Context, query string) (context.Context, trace.Span, attribute.Set) {
	summary := querySummary(query)

	spanAttrs := append(db.commonAttrs.ToSlice(),
		semconv.DBQuerySummary(summary),
		semconv.DBQueryText(query),
	)

	ctx, span := db.tracer.Start(ctx, summary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs...),
	)

	metricAttrs := append(db.commonAttrs.ToSlice(), semconv.DBQuerySummary(summary))
	return ctx, span, attribute.NewSet(metricAttrs...)
}

// endSpan emits an error log (correlated with the span via ctx) and sets the
// span status if err is non-nil, then ends the span.
// A log record is used instead of a span event following the OTel
// exceptions-as-logs specification:
// https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/
func endSpan(ctx context.Context, span trace.Span, err error) {
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.ErrorContext(ctx, "database error",
			slog.String("exception.type", fmt.Sprintf("%T", err)),
			slog.String("exception.message", err.Error()),
		)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// querySummary returns a low-cardinality summary of a SQL query suitable for
// use as a span name and db.query.summary attribute, e.g. "SELECT restaurants".
// The format follows the OTel database span naming convention:
// https://opentelemetry.io/docs/specs/semconv/database/database-spans/#generating-a-summary-of-the-query
func querySummary(query string) string {
	words := strings.Fields(strings.TrimSpace(query))
	if len(words) == 0 {
		return "UNKNOWN"
	}
	op := strings.ToUpper(words[0])
	var table string
	switch op {
	case "SELECT":
		for i, w := range words {
			if strings.EqualFold(w, "FROM") && i+1 < len(words) {
				table = cleanTableName(words[i+1])
				break
			}
		}
	case "INSERT":
		if len(words) >= 3 && strings.EqualFold(words[1], "INTO") {
			table = cleanTableName(words[2])
		}
	case "UPDATE":
		if len(words) >= 2 {
			table = cleanTableName(words[1])
		}
	case "DELETE":
		if len(words) >= 3 && strings.EqualFold(words[1], "FROM") {
			table = cleanTableName(words[2])
		}
	}
	if table != "" {
		return op + " " + table
	}
	return op
}

// cleanTableName strips schema prefixes and trailing punctuation from a table
// token so "public.restaurants," becomes "restaurants".
func cleanTableName(s string) string {
	s = strings.TrimRight(s, ",(")
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return s
}
