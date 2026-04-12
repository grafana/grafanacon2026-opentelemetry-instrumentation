# Exercise 04 — Customizing Instrumentations

[← Exercise 03](03-instrumenting-applications.md) | [Exercise 05 →](05-processing.md)

Drop noisy spans with a custom sampler, suppress instrumentation modules, enrich spans with application context, and filter at the collector when you can't touch application code.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Drop health-check spans](#part-1--drop-health-check-spans)
  - [Step 1 — Create a custom sampler](#step-1--create-a-custom-sampler)
  - [Step 2 — Wire the sampler into the TracerProvider](#step-2--wire-the-sampler-into-the-tracerprovider)
- [Part 2 — Disable noisy auto-instrumentation (Node.js frontend)](#part-2--disable-noisy-auto-instrumentation-nodejs-frontend)
  - [Step 3 — Disable the `net` instrumentation](#step-3--disable-the-net-instrumentation)
- [Part 3 — Enrich spans with user identity](#part-3--enrich-spans-with-user-identity)
  - [Step 4 — Set `enduser.id` on incoming spans](#step-4--set-enduserid-on-incoming-spans)
- [Part 4 — Filter frontend noise in the collector](#part-4--filter-frontend-noise-in-the-collector)
  - [Step 5 — Add a filter processor](#step-5--add-a-filter-processor)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| Service   | File                                                                                                                                                                  | What changes                                                                     |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| backend   | [backend/sampler.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/backend/sampler.go)                 | New file — custom sampler that drops `/api/health` spans                         |
| backend   | [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/backend/telemetry.go)             | Wire the custom sampler into the `TracerProvider`                                |
| frontend  | [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/docker-compose.yaml)               | Disable the `net` auto-instrumentation module                                    |
| frontend  | [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/frontend/server.js)                 | Set `enduser.id` and `enduser.pseudo.id` on every authenticated span             |
| collector | [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/otel-collector/config.yaml) | Filter processor that drops static-file and health-check spans from the frontend |

---

## Part 1 — Drop health-check spans

Health-check endpoints are polled constantly and generate a large volume of low-value spans. A custom sampler drops them before they leave the process.

We implement this for the **Go backend**, which uses manual SDK initialization. The **Node.js frontend** uses zero-code auto-instrumentation, so there is nowhere to wire up a custom sampler. Two alternatives exist:

- **Disable the instrumentation entirely** — no span is created. See [Part 2](#part-2--disable-noisy-auto-instrumentation-nodejs-frontend).
- **Filter in the collector** — see [Part 4](#part-4--filter-frontend-noise-in-the-collector). Only safe for leaf spans; dropping a parent breaks the trace tree.

If you switch the frontend to a [code-based SDK setup](https://opentelemetry.io/docs/languages/js/sampling/) you can pass a custom `Sampler` to `NodeSDK` directly.

### Step 1 — Create a custom sampler

Create [backend/sampler.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/backend/sampler.go):

```go
package main

import (
	"go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// dropHealthSampler drops HTTP server spans for the health endpoint
// and delegates everything else to the wrapped sampler.
type dropHealthSampler struct {
	delegate sdktrace.Sampler
}

func (s dropHealthSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	if p.Kind == trace.SpanKindServer {
		for _, attr := range p.Attributes {
			if attr.Key == "url.path" && attr.Value.AsString() == "/api/health" {
				return sdktrace.SamplingResult{Decision: sdktrace.Drop}
			}
		}
	}
	return s.delegate.ShouldSample(p)
}

func (s dropHealthSampler) Description() string {
	return "DropHealth"
}
```

### Step 2 — Wire the sampler into the TracerProvider

In [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/backend/telemetry.go):

```diff
 tp := sdktrace.NewTracerProvider(
 	sdktrace.WithBatcher(traceExp),
+	sdktrace.WithSampler(dropHealthSampler{
+		delegate: sdktrace.ParentBased(sdktrace.AlwaysSample()),
+	}),
 )
```

`ParentBased(AlwaysSample())` is the standard default: honour the sampling decision from an incoming `traceparent` header, sample everything without a parent. `dropHealthSampler` wraps it and intercepts health-check spans first.

> [!NOTE]
> Sampling happens before the span is created, so only attributes passed as **start options** (via `trace.WithAttributes` at span creation) are visible in `SamplingParameters.Attributes`. `otelmux` sets `url.path` as a start attribute, which is why this sampler can read it. For attributes added after span creation, use a `SpanProcessor` instead.

---

## Part 2 — Disable noisy auto-instrumentation (Node.js frontend)

The `net` module instrumentation produces low-level TCP spans that are rarely useful.

### Step 3 — Disable the `net` instrumentation

In [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/docker-compose.yaml):

```diff
   frontend:
     environment:
       OTEL_SERVICE_NAME: frontend
       OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
+      OTEL_NODE_DISABLED_INSTRUMENTATIONS: net
```

Accepts a comma-separated list, e.g. `net,dns`.

---

## Part 3 — Enrich spans with user identity

Auto-instrumentation knows nothing about session state. Setting attributes on the active span in a middleware enriches every trace with the logged-in user.

### Step 4 — Set `enduser.id` on incoming spans

In [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/frontend/server.js), add the import and extend the existing auth middleware:

```diff
+const { trace } = require('@opentelemetry/api');

 app.use((req, res, next) => {
   try {
     req.currentUser = req.cookies.tapas_user ? JSON.parse(req.cookies.tapas_user) : null;
   } catch {
     req.currentUser = null;
   }
   res.locals.currentUser = req.currentUser;
+
+  if (req.currentUser) {
+    const span = trace.getActiveSpan();
+    if (span) {
+      span.setAttribute('enduser.id', req.currentUser.username);
+      span.setAttribute('enduser.pseudo.id', String(req.currentUser.id));
+    }
+  }
+
   next();
 });
```

`trace.getActiveSpan()` returns the span the auto-instrumentation already created — no manual span needed.

- `enduser.id` — username, useful for searching traces by user.
- `enduser.pseudo.id` — opaque internal DB ID, stable identifier without a recognizable identity.

> [!WARNING]
> `enduser.id` is personal data. Check your privacy policy before capturing it.

---

## Part 4 — Filter frontend noise in the collector

The [filter processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/processor/filterprocessor) uses [OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/pkg/ottl) expressions to drop spans at the collector — useful when you can't modify application code.

> [!WARNING]
> **Only drop leaf spans.** The collector cannot rewrite `parent_span_id` references. Dropping a parent orphans its children, breaking the trace tree. Static file requests and health-check pings are safe targets; for anything else prefer disabling the module ([Part 2](#part-2--disable-noisy-auto-instrumentation-nodejs-frontend)) or a custom sampler ([Part 1](#part-1--drop-health-check-spans)).

### Step 5 — Add a filter processor

In [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/otel-collector/config.yaml):

```diff
 processors:
   resourcedetection:
     detectors: [env, system]
     timeout: 2s
     override: false

   batch:
     timeout: 1s

+  filter/drop_frontend_noise:
+    error_mode: ignore
+    traces:
+      span:
+        - >-
+          resource.attributes["service.name"] == "frontend" and
+          kind == SPAN_KIND_SERVER and
+          (attributes["url.path"] == "/health" or
+          IsMatch(attributes["url.path"], "\\.(css|js|ico|png|jpg|jpeg|gif|svg|woff|woff2|ttf|eot|map)(\\?.*)?$"))

 service:
   pipelines:
     traces:
       receivers: [otlp]
-      processors: [resourcedetection, batch]
+      processors: [resourcedetection, filter/drop_frontend_noise, batch]
       exporters: [otlphttp]
```

The condition drops a span when all three are true: service is `frontend`, span kind is `SERVER`, and `url.path` is `/health` or a static file extension. The processor runs before `batch`, so filtered spans never reach the exporter.

---

## Verify

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

Run the TraceQL queries below — click each link to open Grafana Explore with the query pre-loaded, or paste the query manually into Grafana → **Explore** → **Tempo**.

**Parts 1 & 4 — health-check spans dropped** — should return no results:

[Open in Grafana](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20kind%20%3D%20server%20%26%26%20span.url.path%20%3D~%20%5C%22.%2Ahealth%5C%22%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D)

```traceql
{ kind = server && span.url.path =~ ".*health" }
```

> Note: the APM dashboard may still show a `health` operation — it's built from metrics, which are unaffected.

**Part 2 — `net` spans gone** — should return no results:

[Open in Grafana](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20resource.service.name%20%3D%20%5C%22frontend%5C%22%20%26%26%20name%20%3D%20%5C%22tcp.connect%5C%22%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D)

```traceql
{ resource.service.name = "frontend" && name = "tcp.connect" }
```

**Part 3 — user identity on spans** — log in as any user (e.g. `alice`), then:

[Open in Grafana](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20resource.service.name%20%3D%20%5C%22frontend%5C%22%20%26%26%20kind%20%3D%20server%20%26%26%20span.enduser.id%20%21%3D%20nil%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D)

```traceql
{ resource.service.name = "frontend" && kind = server && span.enduser.id != nil }
```

**Part 4 — static file spans dropped** — should return no results:

[Open in Grafana](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20resource.service.name%20%3D%20%5C%22frontend%5C%22%20%26%26%20kind%20%3D%20server%20%26%26%20span.url.path%20%3D~%20%5C%22.%2A%28css%7Cjs%7Cico%7Cpng%29%5C%22%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D)

```traceql
{ resource.service.name = "frontend" && kind = server && span.url.path =~ ".*(css|js|ico|png)" }
```

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

---

## Catch up

```bash
git checkout origin/04-customizing-instrumentations
```

---

[← Exercise 03](03-instrumenting-applications.md) | [Exercise 05 →](05-processing.md)
