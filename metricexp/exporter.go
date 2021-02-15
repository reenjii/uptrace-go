/*
metricexp provides metric exporter for OpenTelemetry.
*/
package metricexp

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/uptrace-go/internal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

type Exporter struct {
	cfg *Config

	client   internal.SimpleClient
	endpoint string

	mmsc      []mmsc
	quantiles []quantile
}

var _ export.Exporter = (*Exporter)(nil)

func NewExporter(cfg *Config) (*Exporter, error) {
	cfg.init()

	e := &Exporter{
		cfg: cfg,
	}

	dsn, err := internal.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	e.client.Client = cfg.HTTPClient
	e.client.Token = dsn.Token
	e.client.MaxRetries = cfg.MaxRetries

	e.endpoint = fmt.Sprintf("%s://%s/api/v1/projects/%s/metrics",
		dsn.Scheme, dsn.Host, dsn.ProjectID)

	return e, nil
}

// InstallNewPipeline instantiates a NewExportPipeline and registers it globally.
// Typically called as:
//
// 	pipeline := stdout.InstallNewPipeline(stdout.Config{...})
// 	defer pipeline.Stop()
// 	... Done
func InstallNewPipeline(
	ctx context.Context, config *Config, options ...controller.Option,
) (*controller.Controller, error) {
	options = append(options, controller.WithCollectPeriod(10*time.Second))
	ctrl, err := NewExportPipeline(config, options...)
	if err != nil {
		return nil, err
	}

	if err := ctrl.Start(ctx); err != nil {
		return nil, err
	}

	otel.SetMeterProvider(ctrl.MeterProvider())

	return ctrl, nil
}

// NewExportPipeline sets up a complete export pipeline with the recommended setup,
// chaining a NewRawExporter into the recommended selectors and integrators.
func NewExportPipeline(
	cfg *Config, options ...controller.Option,
) (*controller.Controller, error) {
	exporter, err := NewExporter(cfg)
	if err != nil {
		return nil, err
	}

	options = append(options, controller.WithPusher(exporter))

	// Not stateful.
	ctrl := controller.New(
		processor.New(simple.NewWithInexpensiveDistribution(), export.DeltaExportKindSelector()),
		options...,
	)

	return ctrl, nil
}

func (e *Exporter) ExportKindFor(*metric.Descriptor, aggregation.Kind) export.ExportKind {
	return export.DeltaExportKind
}

func (e *Exporter) Export(_ context.Context, checkpointSet export.CheckpointSet) error {
	if e.cfg.Disabled {
		return nil
	}

	if err := e.export(checkpointSet); err != nil {
		return err
	}
	e.flush()

	return nil
}

func (e *Exporter) export(checkpointSet export.CheckpointSet) error {
	return checkpointSet.ForEach(export.DeltaExportKindSelector(), func(record export.Record) error {
		switch agg := record.Aggregation().(type) {
		// case aggregation.Quantile:
		// 	return e.exportQuantile(record, agg)
		case aggregation.MinMaxSumCount:
			return e.exportMMSC(record, agg)
		default:
			// log.Printf("unsupported aggregator type: %T", agg)
			return nil
		}
	})
}

func (e *Exporter) exportMMSC(
	record export.Record, agg aggregation.MinMaxSumCount,
) error {
	var expose mmsc

	if err := exportCommon(record, &expose.baseRecord); err != nil {
		return err
	}

	desc := record.Descriptor()
	numKind := desc.NumberKind()

	min, err := agg.Min()
	if err != nil {
		return err
	}
	expose.Min = float32(min.CoerceToFloat64(numKind))

	max, err := agg.Max()
	if err != nil {
		return err
	}
	expose.Max = float32(max.CoerceToFloat64(numKind))

	sum, err := agg.Sum()
	if err != nil {
		return err
	}
	expose.Sum = sum.CoerceToFloat64(numKind)

	count, err := agg.Count()
	if err != nil {
		return err
	}
	expose.Count = count

	e.mmsc = append(e.mmsc, expose)

	return nil
}

// var quantiles = []float64{0.5, 0.75, 0.9, 0.95, 0.99}

// func (e *Exporter) exportQuantile(record export.Record, agg aggregation.Quantile) error {
// 	var expose quantile

// 	if err := exportCommon(record, &expose.baseRecord); err != nil {
// 		return err
// 	}

// 	desc := record.Descriptor()
// 	numKind := desc.NumberKind()

// 	if agg, ok := agg.(aggregation.Count); ok {
// 		count, err := agg.Count()
// 		if err != nil {
// 			return err
// 		}
// 		expose.Count = count
// 	}

// 	for _, q := range quantiles {
// 		n, err := agg.Quantile(q)
// 		if err != nil {
// 			return err
// 		}
// 		expose.Quantiles = append(expose.Quantiles, float32(n.CoerceToFloat64(numKind)))
// 	}

// 	e.quantiles = append(e.quantiles, expose)

// 	return nil
// }

func exportCommon(record export.Record, expose *baseRecord) error {
	desc := record.Descriptor()

	expose.Name = desc.Name()
	expose.Description = desc.Description()
	expose.NumberKind = int8(desc.NumberKind()) // use string?
	expose.Unit = string(desc.Unit())
	expose.Time = time.Now().UnixNano()

	if iter := record.Labels().Iter(); iter.Len() > 0 {
		attrs := record.Resource().Attributes()
		labels := make([]label.KeyValue, 0, len(attrs)+iter.Len())
		labels = append(labels, attrs...)

		for iter.Next() {
			labels = append(labels, iter.Label())
		}

		expose.Labels = labels
	}

	return nil
}

func (e *Exporter) flush() {
	if len(e.mmsc) == 0 && len(e.quantiles) == 0 {
		return
	}

	go func(mmsc []mmsc, quantiles []quantile) {
		out := make(map[string]interface{})
		if len(mmsc) > 0 {
			out["mmsc"] = mmsc
		}
		if len(quantiles) > 0 {
			out["quantiles"] = quantiles
		}

		if err := e.send(out); err != nil {
			internal.Logger.Printf(context.TODO(), "send failed: %s", err)
		}
	}(e.mmsc, e.quantiles)

	e.mmsc = nil
	e.quantiles = nil
}

func (e *Exporter) send(out interface{}) error {
	ctx := context.Background()

	enc := internal.GetEncoder()
	defer internal.PutEncoder(enc)

	data, err := enc.EncodeZstd(out)
	if err != nil {
		return err
	}

	return e.client.Post(ctx, e.endpoint, data)
}

type baseRecord struct {
	Name        string                 `msgpack:"name"`
	Description string                 `msgpack:"description"`
	Unit        string                 `msgpack:"unit"`
	NumberKind  int8                   `msgpack:"numberKind"`
	Labels      internal.KeyValueSlice `msgpack:"labels"`

	Time int64 `msgpack:"time"`
}

type mmsc struct {
	baseRecord

	Min   float32 `msgpack:"min"`
	Max   float32 `msgpack:"max"`
	Sum   float64 `msgpack:"sum"`
	Count uint64  `msgpack:"count"`
}

func (rec *mmsc) String() string {
	return fmt.Sprintf("name=%s min=%f max=%f sum=%f count=%d",
		rec.Name, rec.Min, rec.Max, rec.Sum, rec.Count)
}

type quantile struct {
	baseRecord

	Count     uint64    `msgpack:"count"`
	Quantiles []float32 `msgpack:"quantiles"`
}

func (rec *quantile) String() string {
	return fmt.Sprintf("name=%s count=%d quantiles=%v",
		rec.Name, rec.Count, rec.Quantiles)
}
