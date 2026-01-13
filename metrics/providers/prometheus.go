package providers

import (
	"context"
	"fmt"
	"net/http"

	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type PrometheusProvider struct {
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
	httpHandler   http.Handler
	resource      *resource.Resource
}

type PrometheusConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
}

func NewPrometheusProvider(ctx context.Context, cfg PrometheusConfig) (*PrometheusProvider, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)

	otel.SetMeterProvider(meterProvider)

	meter := meterProvider.Meter(
		cfg.ServiceName,
		metric.WithInstrumentationVersion(cfg.ServiceVersion),
	)

	return &PrometheusProvider{
		meterProvider: meterProvider,
		meter:         meter,
		httpHandler:   promhttp.Handler(),
		resource:      res,
	}, nil
}

func (p *PrometheusProvider) CreateCounter(desc MetricDescriptor) (Counter, error) {
	counter, err := p.meter.Int64Counter(
		desc.Name,
		metric.WithDescription(desc.Description),
		metric.WithUnit(desc.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter: %w", err)
	}

	return &prometheusCounter{counter: counter}, nil
}

func (p *PrometheusProvider) CreateHistogram(desc MetricDescriptor) (Histogram, error) {
	histogram, err := p.meter.Float64Histogram(
		desc.Name,
		metric.WithDescription(desc.Description),
		metric.WithUnit(desc.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram: %w", err)
	}

	return &prometheusHistogram{histogram: histogram}, nil
}

func (p *PrometheusProvider) CreateGauge(desc MetricDescriptor) (Gauge, error) {
	gauge, err := p.meter.Float64Gauge(
		desc.Name,
		metric.WithDescription(desc.Description),
		metric.WithUnit(desc.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gauge: %w", err)
	}

	return &prometheusGauge{gauge: gauge}, nil
}

func (p *PrometheusProvider) CreateUpDownCounter(desc MetricDescriptor) (UpDownCounter, error) {
	upDownCounter, err := p.meter.Int64UpDownCounter(
		desc.Name,
		metric.WithDescription(desc.Description),
		metric.WithUnit(desc.Unit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create up-down counter: %w", err)
	}

	return &prometheusUpDownCounter{counter: upDownCounter}, nil
}

func (p *PrometheusProvider) HTTPHandler() http.Handler {
	return p.httpHandler
}

func (p *PrometheusProvider) Shutdown(ctx context.Context) error {
	if p.meterProvider != nil {
		return p.meterProvider.Shutdown(ctx)
	}
	return nil
}

type prometheusCounter struct {
	counter metric.Int64Counter
}

func (c *prometheusCounter) Add(ctx context.Context, value int64, labels ...Label) {
	opts := metric.AddOption(metric.WithAttributes(labelsToAttributes(labels)...))
	c.counter.Add(ctx, value, opts)
}

func (c *prometheusCounter) Inc(ctx context.Context, labels ...Label) {
	c.Add(ctx, 1, labels...)
}

type prometheusHistogram struct {
	histogram metric.Float64Histogram
}

func (h *prometheusHistogram) Record(ctx context.Context, value float64, labels ...Label) {
	opts := metric.RecordOption(metric.WithAttributes(labelsToAttributes(labels)...))
	h.histogram.Record(ctx, value, opts)
}

type prometheusGauge struct {
	gauge metric.Float64Gauge
}

func (g *prometheusGauge) Set(ctx context.Context, value float64, labels ...Label) {
	opts := metric.RecordOption(metric.WithAttributes(labelsToAttributes(labels)...))
	g.gauge.Record(ctx, value, opts)
}

func (g *prometheusGauge) Add(ctx context.Context, value float64, labels ...Label) {
	opts := metric.RecordOption(metric.WithAttributes(labelsToAttributes(labels)...))
	g.gauge.Record(ctx, value, opts)
}

type prometheusUpDownCounter struct {
	counter metric.Int64UpDownCounter
}

func (u *prometheusUpDownCounter) Add(ctx context.Context, value int64, labels ...Label) {
	opts := metric.AddOption(metric.WithAttributes(labelsToAttributes(labels)...))
	u.counter.Add(ctx, value, opts)
}

func (u *prometheusUpDownCounter) Inc(ctx context.Context, labels ...Label) {
	u.Add(ctx, 1, labels...)
}

func (u *prometheusUpDownCounter) Dec(ctx context.Context, labels ...Label) {
	u.Add(ctx, -1, labels...)
}

func labelsToAttributes(labels []Label) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, len(labels))
	for i, label := range labels {
		attrs[i] = attribute.String(label.Key, label.Value)
	}
	return attrs
}
