# Exercise 03 — Instrumenting Applications

[← Exercise 02](02-setup-obi.md) | [Exercise 04 →](04-customizing-instrumentations.md)

In this exercise you add OpenTelemetry SDK instrumentation to both the Go backend and the Node.js frontend. Both services will emit traces, metrics, and logs via OTLP to the collector.

- **Frontend (Node.js)** uses [zero-code auto-instrumentation](https://opentelemetry.io/docs/zero-code/js/) — no source changes are needed for traces and metrics. A single `--require` flag at startup loads the OTel SDK and automatically instruments HTTP, DNS, and other built-ins.
- **Backend (Go)** uses manual SDK initialization following the [Go instrumentation guide](https://opentelemetry.io/docs/languages/go/). The SDK must be explicitly wired up in code, but in return you get fine-grained control over providers, exporters, and sampling.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Frontend (Node.js)](#part-1--frontend-nodejs)
  - [Step 1 — Add dependencies](#step-1--add-dependencies-to-frontendpackagejson)
  - [Step 2 — Add the OTel log transport (optional)](#step-2--add-the-otel-log-transport-in-frontendserverjs-optional)
  - [Step 3 — Load auto-instrumentation](#step-3--load-auto-instrumentation-in-frontenddockerfile)
  - [Step 4 — Set env vars](#step-4--set-env-vars-in-docker-composeyml)
- [Part 2 — Backend (Go)](#part-2--backend-go)
  - [Step 5 — Install dependencies](#step-5--install-dependencies)
  - [Step 6 — Create telemetry.go](#step-6--create-backendtelemetrygo)
  - [Step 7 — Update main.go](#step-7--update-backendmaingo)
  - [Step 8 — Set env vars](#step-8--set-env-vars-in-docker-composeyml)
- [Part 3 — Grafana](#part-3--add-the-grafana-dashboard-and-alerts)
  - [Step 9 — Add the Grafana dashboard and alerts](#step-9--add-the-grafana-dashboard-and-alerts)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| Service  | File                                                                                                      | What changes                                                  |
| -------- | --------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| frontend | [frontend/package.json](../frontend/package.json)                                                         | Add OTel packages                                             |
| frontend | [frontend/server.js](../frontend/server.js)                                                               | Add OTel log transport to Winston _(optional)_                |
| frontend | [frontend/Dockerfile](../frontend/Dockerfile)                                                             | Load auto-instrumentation via `--require`                     |
| backend  | [backend/telemetry.go](../backend/telemetry.go)                                                           | New file — sets up OTel trace, metric, and log providers      |
| backend  | [backend/main.go](../backend/main.go)                                                                     | Call `setupTelemetry`; add HTTP middleware                    |
| both     | [docker-compose.yml](../docker-compose.yml)                                                               | Set `OTEL_*` env vars for both services; mount Grafana alerts |
| —        | [grafana/dashboards/apm-dashboard.json](../grafana/dashboards/apm-dashboard.json)                         | New APM dashboard — traces, metrics, and logs                 |
| —        | [grafana/provisioning/alerting/frontend-alerts.yml](../grafana/provisioning/alerting/frontend-alerts.yml) | New alert rules for frontend error rate and latency           |

---

## Part 1 — Frontend (Node.js)

### Step 1 — Add dependencies to [frontend/package.json](../frontend/package.json)

```diff
+    "@opentelemetry/api": "^1.9.0",
+    "@opentelemetry/auto-instrumentations-node": "^0.57.0",
+    "@opentelemetry/winston-transport": "^0.9.0",
```

- `auto-instrumentations-node` automatically instruments HTTP, DNS, and other Node.js built-ins.
- `winston-transport` forwards Winston log records as OTel log records.

### Step 2 — Add the OTel log transport in [frontend/server.js](../frontend/server.js) _(optional)_

> [!NOTE]
> This step is optional. If you skip it, Winston logs will still appear in Loki as unstructured text, but they won't be correlated with traces via the OTel SDK.

```diff
+const { OpenTelemetryTransportV3 } = require('@opentelemetry/winston-transport');

 const logger = winston.createLogger({
   transports: [
     new winston.transports.Console(),
+    new OpenTelemetryTransportV3(), // forward log records to the OTel SDK
   ],
 });
```

### Step 3 — Load auto-instrumentation in [frontend/Dockerfile](../frontend/Dockerfile)

```diff
-CMD ["node", "server.js"]
+CMD ["node", "--require", "@opentelemetry/auto-instrumentations-node/register", "server.js"]
+#                         ^ loads the OTel SDK and auto-instruments HTTP, DNS, etc. before app code runs
```

### Step 4 — Set env vars in [docker-compose.yml](../docker-compose.yml)

```diff
   frontend:
     environment:
+      OTEL_SERVICE_NAME: frontend                            # identifies this service in traces and metrics
+      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318 # where to send telemetry
+      OTEL_SEMCONV_STABILITY_OPT_IN: http                   # use stable HTTP semconv (http.request.method, etc.)
```

---

## Part 2 — Backend (Go)

### Step 5 — Install dependencies

```bash
cd backend
go get go.opentelemetry.io/otel \
       go.opentelemetry.io/otel/sdk \
       go.opentelemetry.io/otel/sdk/log \
       go.opentelemetry.io/otel/sdk/metric \
       go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp \
       go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
       go.opentelemetry.io/contrib/bridges/otelslog \
       go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux
go mod tidy
cd ..
```

### Step 6 — Create [backend/telemetry.go](../backend/telemetry.go)

Set up the three OTel SDK providers and replace the default `slog` logger with an OTel bridge. All exporters use OTLP HTTP and pick up `OTEL_EXPORTER_OTLP_ENDPOINT` from the environment.

```go
func setupTelemetry(ctx context.Context) (func(context.Context) error, error) {
	// Traces
	traceExp, err := otlptracehttp.New(ctx)
	// ...
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Metrics
	metricExp, err := otlpmetrichttp.New(ctx)
	// ...
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(5*time.Second),
		)),
	)
	otel.SetMeterProvider(mp)

	// Logs — bridge slog to OTel
	logExp, err := otlploghttp.New(ctx)
	// ...
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)))
	slog.SetDefault(slog.New(otelslog.NewHandler("backend", otelslog.WithLoggerProvider(lp))))

	return func(ctx context.Context) error { /* shutdown all three */ }, nil
}
```

### Step 7 — Update [backend/main.go](../backend/main.go)

Call `setupTelemetry` at startup and add the gorilla/mux HTTP middleware to create a span for every inbound request:

```diff
+	ctx := context.Background()
+   // initialize SDK providers
+	shutdown, err := setupTelemetry(ctx)
+	if err != nil {
+		slog.Error("failed to setup telemetry", "error", err)
+	} else {
+		defer shutdown(ctx) // flush and shut down exporters on exit
+	}
 	...
 	r := mux.NewRouter()
+   // instrument inbound HTTP requests
+	r.Use(otelmux.Middleware("backend"))
```

### Step 8 — Set env vars in [docker-compose.yml](../docker-compose.yml)

```diff
   backend:
     environment:
+      OTEL_SERVICE_NAME: backend                              # identifies this service in traces and metrics
+      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318 # where to send telemetry
+      OTEL_SEMCONV_STABILITY_OPT_IN: database                 # use stable DB semconv (db.query.summary, etc.)
```

---

## Part 3 — Add the Grafana dashboard and alerts

### Step 9 — Add the Grafana dashboard and alerts

```bash
# copies only these files from the solution branch — does not switch branches
git checkout 03-instrumenting-applications -- grafana/dashboards/apm-dashboard.json
git checkout 03-instrumenting-applications -- grafana/provisioning/alerting/frontend-alerts.yml
```

Also mount the alerting provisioning directory in [docker-compose.yml](../docker-compose.yml):

```diff
   lgtm:
     volumes:
+      - ./grafana/provisioning/alerting:/otel-lgtm/grafana/conf/provisioning/alerting:ro
```

---

## Verify

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

Open <http://localhost:3000/d/apm-dashboard/apm-dashboard>. You should see traces, metrics, and logs from both services.

---

## Catch up

```bash
git checkout 03-instrumenting-applications
```

---

[← Exercise 02](02-setup-obi.md) | [Exercise 04 →](04-customizing-instrumentations.md)
