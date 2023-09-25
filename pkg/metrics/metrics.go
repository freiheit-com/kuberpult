package metrics

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
)

func Init() (metric.MeterProvider, http.Handler, error) {
 
	reg := prometheus.NewPedanticRegistry()

	promExp, err := otelprom.New(otelprom.WithRegisterer(reg), otelprom.WithoutScopeInfo(), otelprom.WithoutTargetInfo())
	if err != nil {
		fmt.Println("Failed to create prom exporter")
		panic(err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExp),
	)

	meter := meterProvider.Meter("go.opentelemetry.io/otel/metric#MultiAsyncExample")

	// This is just a sample of memory stats to record from the Memstats
	heapAlloc, err := meter.Int64ObservableUpDownCounter("heapAllocs")
	if err != nil {
		fmt.Println("failed to register updown counter for heapAllocs")
		panic(err)
	}
	gcCount, err := meter.Int64ObservableCounter("gcCount")
	if err != nil {
		fmt.Println("failed to register counter for gcCount")
		panic(err)
	}

	_, err = meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			memStats := &runtime.MemStats{}
			// This call does work
			runtime.ReadMemStats(memStats)

			as := attribute.NewSet(attribute.String("env", "foo"))

			o.ObserveInt64(heapAlloc, int64(memStats.HeapAlloc), metric.WithAttributeSet(as))
			o.ObserveInt64(gcCount, int64(memStats.NumGC))

			return nil
		},
		heapAlloc,
		gcCount,
	)
	if err != nil {
		fmt.Println("Failed to register callback")
		panic(err)
	}

	return meterProvider, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), nil
}
