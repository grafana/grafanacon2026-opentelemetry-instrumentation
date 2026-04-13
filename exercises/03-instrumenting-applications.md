# Exercise 03 — Instrumenting Applications

[← Exercise 02](02-setup-obi.md) | [Exercise 04 →](04-customizing-instrumentations.md)

In this exercise you add OpenTelemetry SDK instrumentation to both the Go backend and the Node.js frontend. Both services will emit traces, metrics, and logs via OTLP to the collector.

- **Frontend (Node.js)** uses [zero-code auto-instrumentation](https://opentelemetry.io/docs/zero-code/js/) — no source changes are needed for traces and metrics. A single `--require` flag at startup loads the OTel SDK and automatically instruments HTTP, DNS, and other built-ins.
- **Backend (Go)** uses [otelconf](https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/otelconf) — a declarative configuration library. Instead of constructing providers and exporters by hand, you write an `otel-config.yaml` file (embedded into the binary) and call a single `otelconf.NewSDK(...)` function to initialize everything.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Frontend (Node.js)](#part-1--frontend-nodejs)
  - [Step 1 — Add dependencies](#step-1--add-dependencies-to-frontendpackagejson)
  - [Step 2 — Add the OTel log transport (optional)](#step-2--add-the-otel-log-transport-in-frontendserverjs-optional)
  - [Step 3 — Load auto-instrumentation](#step-3--load-auto-instrumentation-in-frontenddockerfile)
  - [Step 4 — Set env vars](#step-4--set-env-vars-in-docker-composeyaml)
- [Part 2 — Backend (Go)](#part-2--backend-go)
  - [Step 5 — Install dependencies](#step-5--install-dependencies)
  - [Step 6 — Create otel-config.yaml](#step-6--create-backendotel-configyaml)
  - [Step 7 — Create telemetry.go](#step-7--create-backendtelemetrygo)
  - [Step 8 — Update main.go](#step-8--update-backendmaingo)
  - [Step 9 — Set env vars](#step-9--set-env-vars-in-docker-composeyaml)
- [Part 3 — Grafana](#part-3--grafana)
  - [Step 10 — Add the Grafana dashboard and alerts](#step-10--add-the-grafana-dashboard-and-alerts)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| Service  | File                                                                                                                                                                                                                | What changes                                                 |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| frontend | [frontend/package.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/package.json)                                                           | Add OTel packages                                            |
| frontend | [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/server.js)                                                                 | Add OTel log transport to Winston _(optional)_               |
| frontend | [frontend/Dockerfile](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/Dockerfile)                                                               | Load auto-instrumentation via `--require`                    |
| backend  | [backend/otel-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/otel-config.yaml)                                                     | New file — declarative OTel SDK configuration                |
| backend  | [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/telemetry.go)                                                             | New file — loads `otel-config.yaml` and wires up SDK globals |
| backend  | [backend/main.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/main.go)                                                                       | Call `setupTelemetry`; add HTTP middleware                   |
| both     | [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/docker-compose.yaml)                                                               | Set `OTEL_*` env vars for both services                      |
| —        | [grafana/dashboards/apm-dashboard.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/grafana/dashboards/apm-dashboard.json)                           | New APM dashboard — traces, metrics, and logs                |
| —        | [grafana/provisioning/alerting/frontend-alerts.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/grafana/provisioning/alerting/frontend-alerts.yaml) | New alert rules for frontend error rate and latency          |

---

## Part 1 — Frontend (Node.js)

### Step 1 — Add dependencies to [frontend/package.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/package.json)

```diff
# frontend/package.json
   "dependencies": {
     "winston": "^3.17.0",
+    "@opentelemetry/api": "^1.9.0",
+    "@opentelemetry/auto-instrumentations-node": "^0.57.0",
+    "@opentelemetry/winston-transport": "^0.9.0",
     "cookie-parser": "^1.4.7",
```

- `auto-instrumentations-node` automatically instruments HTTP, DNS, and other Node.js built-ins.
- `winston-transport` forwards Winston log records as OTel log records.

### Step 2 — Add the OTel log transport in [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/server.js) _(optional)_

> [!NOTE]
> This step is optional. If you skip it, Winston logs will still appear in Loki as unstructured text, but they won't be correlated with traces via the OTel SDK.

```diff
 const winston = require("winston");
+const {
+  OpenTelemetryTransportV3,
+} = require("@opentelemetry/winston-transport");

 const logger = winston.createLogger({
   level: process.env.LOG_LEVEL || "info",
   format: winston.format.json(),
   defaultMeta: { service: "tapas-frontend" },
-  transports: [new winston.transports.Console({ level: "warn" })],
+  transports: [
+    new winston.transports.Console({ level: "warn" }),
+    new OpenTelemetryTransportV3(),
+  ],
 });
```

### Step 3 — Load auto-instrumentation in [frontend/Dockerfile](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/Dockerfile)

```diff
-CMD ["node", "server.js"]
+CMD ["node", "--require", "@opentelemetry/auto-instrumentations-node/register", "server.js"]
```

`--require` executes the module before any other code loads. This is how it can monkey-patch built-in modules like `http` — the patches are in place before the app imports anything.

### Step 4 — Set env vars in [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/docker-compose.yaml)

```diff
   frontend:
     environment:
       BACKEND_URL: http://backend:8080
       PORT: "8080"
       CHAOS_MODE: ${CHAOS_MODE:-}
+      OTEL_SERVICE_NAME: frontend
+      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
+      OTEL_METRIC_EXPORT_INTERVAL: "5000"
+      OTEL_SEMCONV_STABILITY_OPT_IN: http      # opt in to stable HTTP semconv (http.request.method, etc.)
```

---

## Part 2 — Backend (Go)

### Step 5 — Install dependencies

```bash
cd backend
go get go.opentelemetry.io/contrib/otelconf \
       go.opentelemetry.io/contrib/bridges/otelslog \
       go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux
cd ..
```

`otelconf` replaces the individual exporter and SDK packages — it reads a YAML config file and constructs all providers internally.

### Step 6 — Create [backend/otel-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/otel-config.yaml)

This file declares the full SDK configuration — exporters, readers, propagators, and resource attributes. It is embedded into the binary at compile time (see Step 7).

```yaml
file_format: "1.0"

resource:
  attributes:
    - name: service.name
      value: ${OTEL_SERVICE_NAME:-tapas-backend}

tracer_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT:-http://otel-collector:4318}/v1/traces

meter_provider:
  readers:
    - periodic:
        interval: 5000
        exporter:
          otlp_http:
            endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT:-http://otel-collector:4318}/v1/metrics

logger_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT:-http://otel-collector:4318}/v1/logs

propagator:
  composite:
    - tracecontext
    - baggage
```

`${VAR:-default}` substitution is supported throughout the config file — `OTEL_SERVICE_NAME` and `OTEL_EXPORTER_OTLP_ENDPOINT` are read from the environment, with fallbacks if not set.

### Step 7 — Create [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/telemetry.go)

Embed the config file and call `otelconf.NewSDK` to build all providers in one call:

```go
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
```

`//go:embed` bakes the YAML into the binary at compile time — no file path to manage at runtime.

### Step 8 — Update [backend/main.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/main.go)

Call `setupTelemetry` at startup and add the gorilla/mux HTTP middleware to create a span for every inbound request:

```diff
-	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
+	// Fallback logger (console) — replaced by OTel bridge if telemetry setup succeeds.
+	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
+
+	ctx := context.Background()
+	shutdown, err := setupTelemetry(ctx)
+	if err != nil {
+		slog.Error("failed to setup telemetry", "error", err)
+	} else {
+		defer shutdown(ctx)
+	}
 	...
 	r := mux.NewRouter()
+	r.Use(otelmux.Middleware("backend"))
```

Now that all imports are in place, run `go mod tidy` to update the module graph:

```bash
cd backend && go mod tidy && cd ..
```

### Step 9 — Set env vars in [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/docker-compose.yaml)

```diff
   backend:
     environment:
       DB_URL: postgres://postgres:postgres@db:5432/tapas?sslmode=disable
       PORT: "8080"
       CHAOS_MODE: ${CHAOS_MODE:-}
+      OTEL_SERVICE_NAME: backend
+      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
+      OTEL_METRIC_EXPORT_INTERVAL: "5000"
```

`otel-config.yaml` uses `${OTEL_SERVICE_NAME:-tapas-backend}` and `${OTEL_EXPORTER_OTLP_ENDPOINT:-http://otel-collector:4318}` — env vars when set, falling back to the defaults.

---

## Part 3 — Grafana

### Step 10 — Add the Grafana dashboard and alerts

```bash
# copies only these files from the exercise branch — does not switch branches
git checkout origin/03-instrumenting-applications -- grafana/dashboards/apm-dashboard.json
git checkout origin/03-instrumenting-applications -- grafana/provisioning/alerting/frontend-alerts.yaml
```

---

## Verify

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

> [!NOTE]
> Traces, metrics, and logs may take up to a minute to appear after the services start. If the dashboard is empty, wait a moment and refresh.

Open <http://localhost:3000/d/apm-dashboard>. You should see traces, metrics, and logs from both services.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

---

## Catch up

```bash
git checkout origin/03-instrumenting-applications
```

---

[← Exercise 02](02-setup-obi.md) | [Exercise 04 →](04-customizing-instrumentations.md)
