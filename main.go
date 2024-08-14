package main

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
)

func main() {
	provider, err := initProvider()
	if err != nil {
		panic(err)
	}

	metrics := []delayMetric{
		{
			Name:  "delivery.delay",
			Value: float64(5 * time.Second),
			Tags: []Tag{
				{
					Name:  "delivery_id",
					Value: "1",
				},
				{
					Name:  "env",
					Value: "prod",
				},
			},
		},
		{
			Name:  "delivery.delay",
			Value: float64(10 * time.Second),
			Tags: []Tag{
				{
					Name:  "delivery_id",
					Value: "2",
				},
				{
					Name:  "env",
					Value: "prod",
				},
			},
		},
	}
	if err := provider.Gauge(context.Background(), metrics); err != nil {
		panic(err)
	}
}

type expoter struct {
	provider *sdkmetric.MeterProvider
}

func initProvider() (*expoter, error) {
	ctx := context.Background()

	resource, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("my-service"),
			semconv.ServiceVersion("0.1.0"),
		))
	if err != nil {
		return nil, err
	}

	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(time.Hour)),
		),
		sdkmetric.WithResource(resource),
	)
	otel.SetMeterProvider(meterProvider)

	return &expoter{
		provider: meterProvider,
	}, nil
}

type delayMetric struct {
	Name  string
	Value float64
	Tags  []Tag
}

type Tag struct {
	Name  string
	Value string
}

func (r *expoter) Gauge(ctx context.Context, metrics []delayMetric) error {
	for _, m := range metrics {
		var attrs []attribute.KeyValue
		for _, t := range m.Tags {
			attrs = append(attrs, attribute.String(t.Name, t.Value))
		}

		meter := r.provider.Meter(m.Name)
		gauge, err := meter.Float64Gauge(m.Name)
		if err != nil {
			return err
		}
		gauge.Record(ctx, m.Value, metric.WithAttributes(attrs...))
	}
	if err := r.provider.ForceFlush(ctx); err != nil {
		return err
	}

	return nil
}
