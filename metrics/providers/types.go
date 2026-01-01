package providers

import (
	"context"
	"net/http"
	"time"
)

type Label struct {
	Key   string
	Value string
}

func Labels(pairs ...string) []Label {
	if len(pairs)%2 != 0 {
		panic("Labels requires even number of arguments (key-value pairs)")
	}
	labels := make([]Label, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		labels = append(labels, Label{Key: pairs[i], Value: pairs[i+1]})
	}
	return labels
}

type Counter interface {
	Add(ctx context.Context, value int64, labels ...Label)
	Inc(ctx context.Context, labels ...Label)
}

type Histogram interface {
	Record(ctx context.Context, value float64, labels ...Label)
}

type Gauge interface {
	Set(ctx context.Context, value float64, labels ...Label)
	Add(ctx context.Context, value float64, labels ...Label)
}

type UpDownCounter interface {
	Add(ctx context.Context, value int64, labels ...Label)
	Inc(ctx context.Context, labels ...Label)
	Dec(ctx context.Context, labels ...Label)
}

type MetricDescriptor struct {
	Name        string
	Description string
	Unit        string
	Type        MetricType
}

type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeHistogram
	MetricTypeGauge
	MetricTypeUpDownCounter
)

type Provider interface {
	CreateCounter(desc MetricDescriptor) (Counter, error)
	CreateHistogram(desc MetricDescriptor) (Histogram, error)
	CreateGauge(desc MetricDescriptor) (Gauge, error)
	CreateUpDownCounter(desc MetricDescriptor) (UpDownCounter, error)
	HTTPHandler() http.Handler
	Shutdown(ctx context.Context) error
}

type NoopCounter struct{}

func (n *NoopCounter) Add(ctx context.Context, value int64, labels ...Label) {}
func (n *NoopCounter) Inc(ctx context.Context, labels ...Label)              {}

type NoopHistogram struct{}

func (n *NoopHistogram) Record(ctx context.Context, value float64, labels ...Label) {}

type NoopGauge struct{}

func (n *NoopGauge) Set(ctx context.Context, value float64, labels ...Label) {}
func (n *NoopGauge) Add(ctx context.Context, value float64, labels ...Label) {}

type NoopUpDownCounter struct{}

func (n *NoopUpDownCounter) Add(ctx context.Context, value int64, labels ...Label) {}
func (n *NoopUpDownCounter) Inc(ctx context.Context, labels ...Label)              {}
func (n *NoopUpDownCounter) Dec(ctx context.Context, labels ...Label)              {}

type HistogramBuckets struct {
	Boundaries []float64
}

func DefaultHTTPBuckets() HistogramBuckets {
	return HistogramBuckets{
		Boundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}
}

func DefaultDurationBuckets() HistogramBuckets {
	return HistogramBuckets{
		Boundaries: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60},
	}
}

func DurationToSeconds(d time.Duration) float64 {
	return d.Seconds()
}
