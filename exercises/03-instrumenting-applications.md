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
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| File                                                                                                                                                                                                                | Changes                                                         |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| [frontend/package.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/package.json)                                                           | Add OTel packages                                               |
| [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/server.js)                                                                 | Add OTel log transport to Winston _(optional)_                  |
| [frontend/Dockerfile](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/Dockerfile)                                                               | Load auto-instrumentation via `--require`                       |
| [backend/otel-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/otel-config.yaml)                                                     | New file — declarative OTel SDK configuration                   |
| [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/telemetry.go)                                                             | New file — loads `otel-config.yaml` and wires up SDK globals    |
| [backend/main.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/main.go)                                                                       | Call `setupTelemetry`; add HTTP middleware                      |
| [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/docker-compose.yaml)                                                               | Set `OTEL_*` env vars for both services                         |
| [grafana/dashboards/apm-dashboard.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/grafana/dashboards/apm-dashboard.json)                           | Pre-provisioned APM dashboard — traces, metrics, and logs       |
| [grafana/provisioning/alerting/frontend-alerts.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/grafana/provisioning/alerting/frontend-alerts.yaml) | Pre-provisioned alert rules for frontend error rate and latency |

---

## Part 1 — Frontend (Node.js)

### Step 1 — Add dependencies to [frontend/package.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/package.json)

```diff
// frontend/package.json
+    "@opentelemetry/api": "^1.9.0",
+    "@opentelemetry/auto-instrumentations-node": "^0.57.0",
+    "@opentelemetry/winston-transport": "^0.9.0",
```

- `auto-instrumentations-node` automatically instruments HTTP, DNS, and other Node.js built-ins.
- `winston-transport` forwards Winston log records as OTel log records.

### Step 2 — Add the OTel log transport in [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/server.js) _(optional)_

> [!NOTE]
> This step is optional. If you skip it, Winston logs will still appear in Loki as unstructured text, but they won't be correlated with traces via the OTel SDK.

```diff
// frontend/server.js
+const {
+  OpenTelemetryTransportV3,
+} = require("@opentelemetry/winston-transport");

 const logger = winston.createLogger({
   transports: [
-    new winston.transports.Console({ level: "warn" }),
+    new winston.transports.Console({ level: "warn" }),
+    new OpenTelemetryTransportV3(),
   ],
 });
```

### Step 3 — Load auto-instrumentation in [frontend/Dockerfile](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/frontend/Dockerfile)

```diff
# frontend/Dockerfile
-CMD ["node", "server.js"]
+CMD ["node", "--require", "@opentelemetry/auto-instrumentations-node/register", "server.js"]
```

`--require` executes the module before any other code loads. This is how it can monkey-patch built-in modules like `http` — the patches are in place before the app imports anything.

### Step 4 — Set env vars in [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/docker-compose.yaml)

```diff
# docker-compose.yaml
   frontend:
     environment:
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

> [!TIP]
> Manual SDK onboarding — constructing providers, exporters, and processors in code — is an alternative to declarative config. It gives more control but requires more boilerplate. See [Add OpenTelemetry Instrumentation](https://opentelemetry.io/docs/languages/go/getting-started/#add-opentelemetry-instrumentation) in the Go getting-started guide.

### Step 6 — Create [backend/otel-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/otel-config.yaml)

This file declares the full SDK configuration — exporters, readers, propagators, and resource attributes. It is embedded into the binary at compile time (see Step 7).

```yaml
# backend/otel-config.yaml
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
    - tracecontext:
    - baggage:
```

`${VAR:-default}` substitution is supported throughout the config file — `OTEL_SERVICE_NAME` and `OTEL_EXPORTER_OTLP_ENDPOINT` are read from the environment, with fallbacks if not set.

### Step 7 — Create [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/telemetry.go)

Embed the config file and call `otelconf.NewSDK` to build all providers in one call:

```go
// backend/telemetry.go
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
```

> [!NOTE]
> `embed` is imported as `_ "embed"` (blank import) because it's only used by the `//go:embed` directive, not called directly — editors with auto-organize-imports will strip a plain `"embed"` import.

`//go:embed` bakes the YAML into the binary at compile time — no file path to manage at runtime.

### Step 8 — Update [backend/main.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/backend/main.go)

Call `setupTelemetry` at startup and add the gorilla/mux HTTP middleware to create a span for every inbound request:

```diff
// backend/main.go
 import (
+	"context"
 	"fmt"
 	"log"
 	"log/slog"
 	"net/http"
 	"os"

 	"github.com/gorilla/mux"
 	"github.com/rs/cors"
 	dbpkg "github.com/workshop/tapas-backend/db"
 	"github.com/workshop/tapas-backend/handlers"
 	"github.com/workshop/tapas-backend/middleware"
+	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
 )
 ...
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

### Step 9 — Set env vars in [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/03-instrumenting-applications/docker-compose.yaml)

```diff
# docker-compose.yaml
   backend:
     environment:
+      OTEL_SERVICE_NAME: backend
+      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
+      OTEL_METRIC_EXPORT_INTERVAL: "5000"
```

`otel-config.yaml` uses `${OTEL_SERVICE_NAME:-tapas-backend}` and `${OTEL_EXPORTER_OTLP_ENDPOINT:-http://otel-collector:4318}` — env vars when set, falling back to the defaults.

---

## Verify

```bash
docker compose up --build
```

A pre-built APM dashboard lives in [grafana/dashboards/apm-dashboard.json](../grafana/dashboards/apm-dashboard.json) and frontend alerts in [grafana/provisioning/alerting/frontend-alerts.yaml](../grafana/provisioning/alerting/frontend-alerts.yaml). Both are automatically provisioned on startup.

Open <http://localhost:3000/d/apm-dashboard>. You should see traces, metrics, and logs from both services.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

> [!NOTE]
> **OBI backs off once the SDK loads.** Once the services load the OTel SDK, OBI detects it and stops producing metrics for them to avoid double-counting. The RED dashboard from exercise 02 may now look partially empty — in particular, outgoing calls from backend to database disappear, because the SDK doesn't instrument `database/sql` out of the box. HTTP and RPC panels keep working since the SDK covers those (`otelmux`, auto-instrumentations-node). DB instrumentation is added by hand in [exercise 06](06-manual-instrumentation.md); you can also pull in community libraries like [`otelsql`](https://github.com/XSAM/otelsql) instead of writing your own.

---

## Learn more

- [Instrumentation concepts](https://opentelemetry.io/docs/concepts/instrumentation/) — zero-code, code-based, and manual instrumentation explained
- [Go language guide](https://opentelemetry.io/docs/languages/go/) — SDK setup, APIs, and instrumentation libraries for Go
- [Node.js zero-code instrumentation](https://opentelemetry.io/docs/zero-code/js/) — `--require` loader and supported libraries
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) — standard attribute names for HTTP, RPC, resources, and more
- [W3C Trace Context](https://www.w3.org/TR/trace-context-2/) — the propagation format used by `tracecontext`
- [OpenTelemetry Registry](https://opentelemetry.io/ecosystem/registry/) and [Ecosystem Explorer](https://explorer.opentelemetry.io) — find instrumentation libraries by language and framework

---

## Catch up

```bash
git checkout origin/03-instrumenting-applications
```

---

[← Exercise 02](02-setup-obi.md) | [Exercise 04 →](04-customizing-instrumentations.md)
