package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type OTelShutdownFunc func(ctx context.Context) error

var (
	initOnce     sync.Once
	initShutdown OTelShutdownFunc
	initErr      error
)

// secondsBuckets are the histogram boundaries for the *_seconds instruments.
// The SDK default (0, 5, 10, 25 ... 10000) is shaped for milliseconds and
// would put every request in the first bucket.
var secondsBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120}

// InitOTel wires the metric and trace providers. The Prometheus exporter is
// always installed, so /metrics serves the prism_* instruments whether or not
// an OTLP collector exists; OTEL_ENABLED=true adds OTLP export for traces and
// metrics. It runs once per process; later calls return the first result.
func InitOTel(ctx context.Context) (OTelShutdownFunc, error) {
	initOnce.Do(func() {
		initShutdown, initErr = initOTel(ctx)
	})
	return initShutdown, initErr
}

func initOTel(ctx context.Context) (OTelShutdownFunc, error) {
	enabledRaw := os.Getenv("OTEL_ENABLED")
	otlpEnabled := enabledRaw == "true" || enabledRaw == "1"

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "prism-gateway"
	}
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "http://localhost:4318"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("0.4.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	// Metrics: the Prometheus exporter registers into the default registry
	// that promhttp serves. WithoutUnits keeps prism_cost_usd_total from
	// becoming prism_cost_usd_USD_total; WithoutScopeInfo keeps the series
	// free of otel_scope_* labels.
	promExporter, err := prometheus.New(prometheus.WithoutUnits(), prometheus.WithoutScopeInfo())
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}
	metricOpts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "prism_*_seconds", Kind: sdkmetric.InstrumentKindHistogram},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: secondsBuckets}},
		)),
	}

	var shutdowns []func(context.Context) error

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if otlpEnabled {
		samplingRate := 1.0
		if rateStr := os.Getenv("OTEL_SAMPLING_RATE"); rateStr != "" {
			if rate, err := strconv.ParseFloat(rateStr, 64); err == nil {
				samplingRate = rate
			}
		}
		traceOpts := []sdktrace.TracerProviderOption{
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(samplingRate)),
		}
		traceExporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(otlpEndpoint+"/v1/traces"),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			log.Printf("otel: OTLP trace exporter not started: %v", err)
		} else {
			traceOpts = append(traceOpts, sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(2*time.Second)))
		}
		tp := sdktrace.NewTracerProvider(traceOpts...)
		otel.SetTracerProvider(tp)
		shutdowns = append(shutdowns, tp.Shutdown)

		otlpMetricExporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpointURL(otlpEndpoint+"/v1/metrics"),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			log.Printf("otel: OTLP metric exporter not started: %v", err)
		} else {
			metricOpts = append(metricOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpMetricExporter, sdkmetric.WithInterval(5*time.Second))))
		}
	}

	mp := sdkmetric.NewMeterProvider(metricOpts...)
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)

	if otlpEnabled {
		log.Printf("otel: metrics on /metrics, traces and metrics exported to %s (service %s)", otlpEndpoint, serviceName)
	} else {
		log.Printf("otel: metrics on /metrics; OTLP export off (set OTEL_ENABLED=true to export traces and metrics)")
	}

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		for _, fn := range shutdowns {
			if err := fn(shutdownCtx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	return shutdown, nil
}
