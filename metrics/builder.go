package metrics

import "github.com/yadunandan004/scaffold/metrics/providers"

type CounterBuilder struct {
	name        string
	description string
	unit        string
	registry    *MetricRegistry
}

func NewCounterBuilder(name string, registry *MetricRegistry) *CounterBuilder {
	return &CounterBuilder{
		name:     name,
		registry: registry,
		unit:     "1",
	}
}

func (b *CounterBuilder) WithDescription(description string) *CounterBuilder {
	b.description = description
	return b
}

func (b *CounterBuilder) WithUnit(unit string) *CounterBuilder {
	b.unit = unit
	return b
}

func (b *CounterBuilder) Build() (providers.Counter, error) {
	return b.registry.RegisterCounter(b.name, b.description, b.unit)
}

func (b *CounterBuilder) MustBuild() providers.Counter {
	counter, err := b.Build()
	if err != nil {
		panic(err)
	}
	return counter
}

type HistogramBuilder struct {
	name        string
	description string
	unit        string
	registry    *MetricRegistry
}

func NewHistogramBuilder(name string, registry *MetricRegistry) *HistogramBuilder {
	return &HistogramBuilder{
		name:     name,
		registry: registry,
		unit:     "1",
	}
}

func (b *HistogramBuilder) WithDescription(description string) *HistogramBuilder {
	b.description = description
	return b
}

func (b *HistogramBuilder) WithUnit(unit string) *HistogramBuilder {
	b.unit = unit
	return b
}

func (b *HistogramBuilder) Build() (providers.Histogram, error) {
	return b.registry.RegisterHistogram(b.name, b.description, b.unit)
}

func (b *HistogramBuilder) MustBuild() providers.Histogram {
	histogram, err := b.Build()
	if err != nil {
		panic(err)
	}
	return histogram
}

type GaugeBuilder struct {
	name        string
	description string
	unit        string
	registry    *MetricRegistry
}

func NewGaugeBuilder(name string, registry *MetricRegistry) *GaugeBuilder {
	return &GaugeBuilder{
		name:     name,
		registry: registry,
		unit:     "1",
	}
}

func (b *GaugeBuilder) WithDescription(description string) *GaugeBuilder {
	b.description = description
	return b
}

func (b *GaugeBuilder) WithUnit(unit string) *GaugeBuilder {
	b.unit = unit
	return b
}

func (b *GaugeBuilder) Build() (providers.Gauge, error) {
	return b.registry.RegisterGauge(b.name, b.description, b.unit)
}

func (b *GaugeBuilder) MustBuild() providers.Gauge {
	gauge, err := b.Build()
	if err != nil {
		panic(err)
	}
	return gauge
}

type UpDownCounterBuilder struct {
	name        string
	description string
	unit        string
	registry    *MetricRegistry
}

func NewUpDownCounterBuilder(name string, registry *MetricRegistry) *UpDownCounterBuilder {
	return &UpDownCounterBuilder{
		name:     name,
		registry: registry,
		unit:     "1",
	}
}

func (b *UpDownCounterBuilder) WithDescription(description string) *UpDownCounterBuilder {
	b.description = description
	return b
}

func (b *UpDownCounterBuilder) WithUnit(unit string) *UpDownCounterBuilder {
	b.unit = unit
	return b
}

func (b *UpDownCounterBuilder) Build() (providers.UpDownCounter, error) {
	return b.registry.RegisterUpDownCounter(b.name, b.description, b.unit)
}

func (b *UpDownCounterBuilder) MustBuild() providers.UpDownCounter {
	upDownCounter, err := b.Build()
	if err != nil {
		panic(err)
	}
	return upDownCounter
}
