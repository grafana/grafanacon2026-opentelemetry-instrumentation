package main

import (
	"context"
	_ "embed"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/otelconf"
	"go.opentelemetry.io/otel"
)

//go:embed otel-config.yaml
var otelConfig []byte

func setupTelemetry(_ context.Context) (func(context.Context) error, error) {
	c, err := otelconf.ParseYAML(otelConfig)
	if err != nil {
		return nil, err
	}
	sdk, err := otelconf.NewSDK(otelconf.WithOpenTelemetryConfiguration(*c))
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(sdk.TracerProvider())
	otel.SetMeterProvider(sdk.MeterProvider())
	otel.SetTextMapPropagator(sdk.Propagator())

	slog.SetDefault(slog.New(otelslog.NewHandler("backend",
		otelslog.WithLoggerProvider(sdk.LoggerProvider()))))

	return sdk.Shutdown, nil
}
