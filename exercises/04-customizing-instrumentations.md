# Exercise 04 — Customizing Instrumentations

[← Exercise 03](03-instrumenting-applications.md) | [Exercise 05 →](05-processing.md)

Suppress noisy auto-instrumentation modules, enrich spans with application context, reduce trace volume with head-based sampling, and drop health-check spans with an instrumentation filter.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Disable noisy auto-instrumentation (Node.js frontend)](#part-1--disable-noisy-auto-instrumentation-nodejs-frontend)
  - [Step 1 — Disable the `net` instrumentation](#step-1--disable-the-net-instrumentation)
- [Part 2 — Enrich spans with user identity](#part-2--enrich-spans-with-user-identity)
  - [Step 2 — Set `enduser.id` on incoming spans](#step-2--set-enduserid-on-incoming-spans)
- [Part 3 — Sample traces at the source](#part-3--sample-traces-at-the-source)
  - [Step 3 — Sample 50% of frontend traces](#step-3--sample-50-of-frontend-traces)
- [Part 4 — Drop health-check spans](#part-4--drop-health-check-spans)
  - [Step 4 — Filter health-check requests in the middleware](#step-4--filter-health-check-requests-in-the-middleware)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| File                                                                                                                                                    | Changes                                                              |
| ------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/docker-compose.yaml) | Disable `net` auto-instrumentation and enable 50% head-based sampler |
| [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/frontend/server.js)   | Set `enduser.id` and `enduser.pseudo.id` on every authenticated span |
| [backend/main.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/backend/main.go)         | Add `WithFilter` to the `otelmux` middleware to skip `/api/health`   |

---

## Part 1 — Disable noisy auto-instrumentation (Node.js frontend)

The `net` module instrumentation produces low-level TCP spans that are rarely useful.

### Step 1 — Disable the `net` instrumentation

In [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/docker-compose.yaml):

```diff
# docker-compose.yaml
   frontend:
     environment:
       OTEL_SERVICE_NAME: frontend
       OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
+      OTEL_NODE_DISABLED_INSTRUMENTATIONS: net
```

Accepts a comma-separated list, e.g. `net,dns`.

---

## Part 2 — Enrich spans with user identity

Auto-instrumentation knows nothing about session state. Setting attributes on the active span in a middleware enriches every trace with the logged-in user.

### Step 2 — Set `enduser.id` on incoming spans

In [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/frontend/server.js), add the import and extend the existing auth middleware:

```diff
// frontend/server.js
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

## Part 3 — Sample traces at the source

Filters and disabled modules target specific known-noisy spans. Sampling is the general-purpose volume knob: the SDK decides at trace start whether to record, and a head-based decision propagates to downstream services via the trace context, so the whole trace is kept or dropped together.

### Step 3 — Sample 50% of frontend traces

In [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/docker-compose.yaml):

```diff
# docker-compose.yaml
   frontend:
     environment:
       OTEL_SERVICE_NAME: frontend
       OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
       OTEL_NODE_DISABLED_INSTRUMENTATIONS: net
+      OTEL_TRACES_SAMPLER: parentbased_traceidratio
+      OTEL_TRACES_SAMPLER_ARG: "0.5"
```

- `parentbased_traceidratio` honors an upstream sampling decision when one exists, and otherwise samples root spans at the configured ratio.
- `OTEL_TRACES_SAMPLER_ARG=0.5` keeps 50% — a deterministic function of the trace ID, so the same trace is always kept or always dropped.

Backend spans inherit the frontend's decision through the `traceparent` header. Spans the backend starts itself (its own health loop, background jobs) are governed by the backend's own sampler — unchanged here.

> [!WARNING]
> Head-based sampling drops errors at the same rate as normal traffic. When error traces matter more than steady-state traces, tail sampling in the collector is the right tool — it waits until the trace ends before deciding.

---

## Part 4 — Drop health-check spans

Health-check endpoints are polled constantly and generate a large volume of low-value spans. The cleanest way to suppress them is an instrumentation-level filter: no span object is allocated and no context is propagated, so there is zero overhead for the dropped requests.

The `otelmux` middleware accepts a `WithFilter` option — a predicate `func(*http.Request) bool` that returns `false` to skip tracing for a request entirely.

For the **Node.js frontend**, which uses zero-code auto-instrumentation, there is no equivalent hook. Two alternatives exist:

- **Disable the instrumentation entirely** — no span is created. See [Part 1](#part-1--disable-noisy-auto-instrumentation-nodejs-frontend).
- **Filter in the collector** — see [Exercise 05](05-processing.md). Only safe for leaf spans; dropping a parent breaks the trace tree.

### Step 4 — Filter health-check requests in the middleware

In [backend/main.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/04-customizing-instrumentations/backend/main.go), add a filter to the existing middleware line:

```diff
// backend/main.go
-r.Use(otelmux.Middleware("backend"))
+r.Use(otelmux.Middleware("backend",
+    otelmux.WithFilter(func(r *http.Request) bool {
+        return r.URL.Path != "/api/health"
+    }),
+))
```

The filter receives the raw `*http.Request` before any span is created. Returning `false` skips the span entirely — more efficient than a sampler (which still allocates the span, then drops it) or a `SpanProcessor` (which runs after the span ends and can only suppress export, not creation).

---

## Verify

```bash
docker compose up --build
```

Paste each query into Grafana → **Explore** → **Tempo**.

**Query A — Part 1, `net` spans gone** — should return no results:

```traceql
{ resource.service.name = "frontend" && name = "tcp.connect" }
```

**Query B — Part 2, user identity on spans** — log in as any user (e.g. `alice`), then:

```traceql
{ resource.service.name = "frontend" && kind = server && span.enduser.id != nil }
```

**Query C — Part 3, ~50% of frontend traces kept** — generate a steady load, pick a fixed time window, and compare the frontend server span count to a pre-sampler run:

```traceql
{ resource.service.name = "frontend" && kind = server }
```

The count should land near half. Sampling is probabilistic, so don't expect an exact ratio from a small sample.

**Query D — Part 4, backend health-check spans dropped** — should return no results:

```traceql
{ resource.service.name = "backend" && kind = server && span.url.path = "/api/health" }
```

> Note: the APM dashboard may still show a `health` operation — it's built from metrics, which are unaffected. The frontend's own `/health` spans will still appear until [Exercise 05](05-processing.md) filters them in the collector.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

---

## Learn more

- [Node.js zero-code instrumentation](https://opentelemetry.io/docs/zero-code/js/) — `OTEL_NODE_DISABLED_INSTRUMENTATIONS` and related knobs
- [OTel sampling](https://opentelemetry.io/docs/concepts/sampling/) — head vs tail sampling, parent-based samplers, and the `OTEL_TRACES_SAMPLER` env vars
- [JavaScript sampling](https://opentelemetry.io/docs/languages/js/sampling/) — programmatic sampler configuration in the Node SDK
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) — including `enduser.*` attributes and the privacy considerations around them

---

## Catch up

```bash
git checkout origin/04-customizing-instrumentations
```

---

[← Exercise 03](03-instrumenting-applications.md) | [Exercise 05 →](05-processing.md)
