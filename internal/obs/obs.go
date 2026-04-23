package obs

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Obs struct {
	cfg            *Config
	tp             *sdktrace.TracerProvider
	SearchDuration *prometheus.HistogramVec
	PageRequests   *prometheus.CounterVec
	ScanProgress   prometheus.Gauge
	ArchiveReads   prometheus.Counter
}

func New(cfg *Config) (*Obs, error) {
	o := &Obs{cfg: cfg}
	o.SearchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mcp_wiki_search_duration_seconds",
		Help:    "Search request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"search_type"})
	o.PageRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_wiki_page_requests_total",
		Help: "Total page requests",
	}, []string{"mode", "cache_hit"})
	o.ScanProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_wiki_scan_progress_total",
		Help: "Articles indexed so far",
	})
	o.ArchiveReads = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mcp_wiki_archive_reads_total",
		Help: "Total archive reads",
	})

	if cfg.OTELEndpoint != "" {
		exp, err := otlptracegrpc.New(context.Background(),
			otlptracegrpc.WithEndpoint(cfg.OTELEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create otel exporter: %w", err)
		}
		res, _ := resource.New(context.Background(),
			resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
		)
		o.tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
	} else {
		o.tp = sdktrace.NewTracerProvider()
	}
	otel.SetTracerProvider(o.tp)

	return o, nil
}

func (o *Obs) Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func (o *Obs) ServeMetrics() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", o.cfg.PrometheusPort)
	zap.L().Info("prometheus metrics listening", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		zap.L().Error("metrics server error", zap.Error(err))
	}
}

func (o *Obs) Shutdown(ctx context.Context) {
	if o.tp != nil {
		_ = o.tp.Shutdown(ctx)
	}
}
