package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/yadunandan004/scaffold/metrics/providers"
)

type MetricRegistry struct {
	provider       providers.Provider
	mu             sync.RWMutex
	counters       map[string]providers.Counter
	histograms     map[string]providers.Histogram
	gauges         map[string]providers.Gauge
	upDownCounters map[string]providers.UpDownCounter
}

func NewMetricRegistry(provider providers.Provider) *MetricRegistry {
	return &MetricRegistry{
		provider:       provider,
		counters:       make(map[string]providers.Counter),
		histograms:     make(map[string]providers.Histogram),
		gauges:         make(map[string]providers.Gauge),
		upDownCounters: make(map[string]providers.UpDownCounter),
	}
}

func (r *MetricRegistry) RegisterCounter(name, description, unit string) (providers.Counter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, found := r.counters[name]; found {
		return existing, nil
	}

	counter, err := r.provider.CreateCounter(providers.MetricDescriptor{
		Name:        name,
		Description: description,
		Unit:        unit,
		Type:        providers.MetricTypeCounter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create counter %s: %w", name, err)
	}

	r.counters[name] = counter
	return counter, nil
}

func (r *MetricRegistry) GetCounter(name string) (providers.Counter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counter, found := r.counters[name]
	return counter, found
}

func (r *MetricRegistry) MustRegisterCounter(name, description, unit string) providers.Counter {
	counter, err := r.RegisterCounter(name, description, unit)
	if err != nil {
		panic(fmt.Sprintf("failed to register counter %s: %v", name, err))
	}
	return counter
}

func (r *MetricRegistry) RegisterHistogram(name, description, unit string) (providers.Histogram, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, found := r.histograms[name]; found {
		return existing, nil
	}

	histogram, err := r.provider.CreateHistogram(providers.MetricDescriptor{
		Name:        name,
		Description: description,
		Unit:        unit,
		Type:        providers.MetricTypeHistogram,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram %s: %w", name, err)
	}

	r.histograms[name] = histogram
	return histogram, nil
}

func (r *MetricRegistry) GetHistogram(name string) (providers.Histogram, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	histogram, found := r.histograms[name]
	return histogram, found
}

func (r *MetricRegistry) MustRegisterHistogram(name, description, unit string) providers.Histogram {
	histogram, err := r.RegisterHistogram(name, description, unit)
	if err != nil {
		panic(fmt.Sprintf("failed to register histogram %s: %v", name, err))
	}
	return histogram
}

func (r *MetricRegistry) RegisterGauge(name, description, unit string) (providers.Gauge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, found := r.gauges[name]; found {
		return existing, nil
	}

	gauge, err := r.provider.CreateGauge(providers.MetricDescriptor{
		Name:        name,
		Description: description,
		Unit:        unit,
		Type:        providers.MetricTypeGauge,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gauge %s: %w", name, err)
	}

	r.gauges[name] = gauge
	return gauge, nil
}

func (r *MetricRegistry) GetGauge(name string) (providers.Gauge, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	gauge, found := r.gauges[name]
	return gauge, found
}

func (r *MetricRegistry) MustRegisterGauge(name, description, unit string) providers.Gauge {
	gauge, err := r.RegisterGauge(name, description, unit)
	if err != nil {
		panic(fmt.Sprintf("failed to register gauge %s: %v", name, err))
	}
	return gauge
}

func (r *MetricRegistry) RegisterUpDownCounter(name, description, unit string) (providers.UpDownCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, found := r.upDownCounters[name]; found {
		return existing, nil
	}

	upDownCounter, err := r.provider.CreateUpDownCounter(providers.MetricDescriptor{
		Name:        name,
		Description: description,
		Unit:        unit,
		Type:        providers.MetricTypeUpDownCounter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create up-down counter %s: %w", name, err)
	}

	r.upDownCounters[name] = upDownCounter
	return upDownCounter, nil
}

func (r *MetricRegistry) GetUpDownCounter(name string) (providers.UpDownCounter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	upDownCounter, found := r.upDownCounters[name]
	return upDownCounter, found
}

func (r *MetricRegistry) MustRegisterUpDownCounter(name, description, unit string) providers.UpDownCounter {
	upDownCounter, err := r.RegisterUpDownCounter(name, description, unit)
	if err != nil {
		panic(fmt.Sprintf("failed to register up-down counter %s: %v", name, err))
	}
	return upDownCounter
}

func (r *MetricRegistry) HTTPHandler() http.Handler {
	return r.provider.HTTPHandler()
}

func (r *MetricRegistry) Shutdown(ctx context.Context) error {
	return r.provider.Shutdown(ctx)
}

func (r *MetricRegistry) MetricCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.counters) + len(r.histograms) + len(r.gauges) + len(r.upDownCounters)
}

func (r *MetricRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters = make(map[string]providers.Counter)
	r.histograms = make(map[string]providers.Histogram)
	r.gauges = make(map[string]providers.Gauge)
	r.upDownCounters = make(map[string]providers.UpDownCounter)
}
