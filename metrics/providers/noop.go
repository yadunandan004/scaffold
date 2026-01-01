package providers

import (
	"context"
	"net/http"
)

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (n *NoopProvider) CreateCounter(desc MetricDescriptor) (Counter, error) {
	return &NoopCounter{}, nil
}

func (n *NoopProvider) CreateHistogram(desc MetricDescriptor) (Histogram, error) {
	return &NoopHistogram{}, nil
}

func (n *NoopProvider) CreateGauge(desc MetricDescriptor) (Gauge, error) {
	return &NoopGauge{}, nil
}

func (n *NoopProvider) CreateUpDownCounter(desc MetricDescriptor) (UpDownCounter, error) {
	return &NoopUpDownCounter{}, nil
}

func (n *NoopProvider) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Noop metrics provider\n"))
	})
}

func (n *NoopProvider) Shutdown(ctx context.Context) error {
	return nil
}
