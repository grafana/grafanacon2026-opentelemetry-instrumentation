# Exercise 05 — Processing telemetry

[← Exercise 04](04-customizing-instrumentations.md) | [Exercise 06 →](06-manual-instrumentation.md)

Filter noisy spans in the collector, use an OTTL transform processor to anonymize sensitive span attributes, and swap head-based sampling for tail-based sampling that keeps every error and every slow trace.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Filter frontend noise in the collector](#part-1--filter-frontend-noise-in-the-collector)
  - [Step 1 — Add a filter processor](#step-1--add-a-filter-processor)
  - [Step 2 — Wire the processor into the traces pipeline](#step-2--wire-the-processor-into-the-traces-pipeline)
- [Part 2 — Anonymize `enduser.id`](#part-2--anonymize-enduserid)
  - [Step 3 — Add a transform processor for traces](#step-3--add-a-transform-processor-for-traces)
  - [Step 4 — Wire the processor into the traces pipeline](#step-4--wire-the-processor-into-the-traces-pipeline)
- [Part 3 — Tail-based sampling in the collector](#part-3--tail-based-sampling-in-the-collector)
  - [Step 5 — Turn off head-based sampling](#step-5--turn-off-head-based-sampling)
  - [Step 6 — Add a tail-sampling processor](#step-6--add-a-tail-sampling-processor)
  - [Step 7 — Wire the processor into the traces pipeline](#step-7--wire-the-processor-into-the-traces-pipeline)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| File                                                                                                                                                | Changes                                                       |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml) | Drop static-file and `/health` spans from the frontend        |
| [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml) | Replace `enduser.id` with a short irreversible hash           |
| [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml) | Keep only error and slow traces via `tail_sampling`           |
| [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/docker-compose.yaml)               | Remove the frontend's head sampler so the collector sees 100% |

---

## Part 1 — Filter frontend noise in the collector

[Exercise 04](04-customizing-instrumentations.md) dropped backend health-check spans at the instrumentation layer and disabled the `net` auto-instrumentation outright — both require application-level access. When you can't touch the application (third-party library, another team's service, a vendor agent), the [filter processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.149.0/processor/filterprocessor) uses [OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.149.0/pkg/ottl) expressions to drop spans at the collector instead.

> [!WARNING]
> **Only drop leaf spans.** The collector cannot rewrite `parent_span_id` references. Dropping a parent orphans its children, breaking the trace tree. Static file requests and health-check pings are safe targets; for anything else prefer disabling the module or an instrumentation filter in the application.

### Step 1 — Add a filter processor

In [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml):

```diff
# otel-collector/config.yaml
+  filter/drop_frontend_noise:
+    error_mode: ignore
+    traces:
+      span:
+        - >-
+          resource.attributes["service.name"] == "frontend" and
+          kind == SPAN_KIND_SERVER and
+          (attributes["url.path"] == "/health" or
+          IsMatch(attributes["url.path"], "\\.(css|js|ico|png|jpg|jpeg|gif|svg|woff|woff2|ttf|eot|map)(\\?.*)?$"))
```

The condition drops a span when all three are true: service is `frontend`, span kind is `SERVER`, and `url.path` is `/health` or a static file extension.

### Step 2 — Wire the processor into the traces pipeline

```diff
# otel-collector/config.yaml
     traces:
       receivers: [otlp]
-      processors: [resourcedetection, batch]
+      processors: [resourcedetection, filter/drop_frontend_noise, batch]
       exporters: [otlp_http]
```

The processor runs before `batch`, so filtered spans never reach the exporter.

---

## Part 2 — Anonymize `enduser.id`

[Exercise 04](04-customizing-instrumentations.md) sets `enduser.id` (username) on spans so traces are searchable by user. Usernames are personal data — a liability if traces are shipped to a third-party backend or retained long-term.

The [transform processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.149.0/processor/transformprocessor) rewrites attributes using OTTL expressions without touching application code.

> [!NOTE]
> `enduser.id` is personal data. The hashing technique below applies to any sensitive string attribute you need to retain but anonymize — useful when you cannot change the application code.

Replace `enduser.id` with the first 8 hex characters of its SHA-256 digest.

### Step 3 — Add a transform processor for traces

In [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml):

```diff
# otel-collector/config.yaml
+  transform/anonymize_enduser:
+    error_mode: ignore
+    trace_statements:
+      - context: span
+        statements:
+          - set(attributes["enduser.id"], Substring(SHA256(attributes["enduser.id"]), 0, 8)) where attributes["enduser.id"] != nil
```

`SHA256()` returns a 64-character hex digest; `Substring(str, start, length)` trims it to 8. The `where` guard skips spans without `enduser.id`. `error_mode: ignore` passes the span through unchanged if the expression fails.

### Step 4 — Wire the processor into the traces pipeline

```diff
# otel-collector/config.yaml
     traces:
       receivers: [otlp]
-      processors: [resourcedetection, filter/drop_frontend_noise, batch]
+      processors: [resourcedetection, filter/drop_frontend_noise, transform/anonymize_enduser, batch]
       exporters: [otlp_http]
```

Place it after the filter — no point hashing attributes on spans that are about to be dropped.

---

## Part 3 — Tail-based sampling in the collector

[Exercise 04](04-customizing-instrumentations.md#part-3--sample-traces-at-the-source) halved trace volume with a head-based sampler — the SDK decides at trace start based only on the trace ID. It's cheap but uninformed: the decision is made before any error or latency signal exists, so a fixed fraction of incidents is dropped unconditionally.

The [tail sampling processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.149.0/processor/tailsamplingprocessor) defers the decision until the trace is observable. The collector buffers every span of a trace in memory and, after a `decision_wait` window elapses, evaluates policies (e.g. status code, latency) against the trace and exports the ones that match.

### Step 5 — Turn off head-based sampling

In [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/docker-compose.yaml), remove the frontend's head sampler so every trace reaches the collector:

```diff
# docker-compose.yaml
   frontend:
     environment:
       OTEL_NODE_DISABLED_INSTRUMENTATIONS: net
-      OTEL_TRACES_SAMPLER: parentbased_traceidratio
-      OTEL_TRACES_SAMPLER_ARG: "0.5"
```

The SDK now falls back to the default `parentbased_always_on` — every root span is sampled, and children inherit the decision. The backend's sampler is already at the default, so nothing changes there.

### Step 6 — Add a tail-sampling processor

In [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml):

```diff
# otel-collector/config.yaml
+  tail_sampling:
+    decision_wait: 10s
+    num_traces: 50000
+    expected_new_traces_per_sec: 100
+    policies:
+      - name: errors
+        type: status_code
+        status_code:
+          status_codes: [ERROR]
+      - name: slow_traces
+        type: latency
+        latency:
+          threshold_ms: 1000
```

Top-level policies are OR-ed: a trace is kept if **any** policy matches. With the two above, every error trace and every trace over 1 s is kept; everything else is dropped outright — no probabilistic fallback.

- `decision_wait: 10s` — how long to buffer before deciding. Must exceed your slowest expected trace, or late spans arrive after the decision and get dropped.
- `num_traces: 50000`, `expected_new_traces_per_sec: 100` — buffer sizing. Tail sampling trades RAM for smarter decisions; tune these for your workload.

### Step 7 — Wire the processor into the traces pipeline

```diff
# otel-collector/config.yaml
     traces:
       receivers: [otlp]
-      processors: [resourcedetection, filter/drop_frontend_noise, transform/anonymize_enduser, batch]
+      processors: [resourcedetection, filter/drop_frontend_noise, tail_sampling, transform/anonymize_enduser, batch]
       exporters: [otlp_http]
```

Order matters: `filter` runs first (cheap drop — no point buffering spans we'll discard anyway), then `tail_sampling` makes the keep/drop call on the rest, and `transform/anonymize_enduser` runs **after** sampling so the SHA-256 hash only runs on traces that actually get exported.

---

## Verify

```bash
docker compose up --build
```

Make sure `CHAOS_MODE=true` is set in your `.env` (it's the default) — chaos mode produces the error and slow traces that tail sampling is meant to keep. Log in as `alice` and exercise the app (restaurant list, detail pages, search). Wait ~15 s after your last click so the `decision_wait` window closes and buffered traces can be emitted.

**Part 1** — static-file and frontend `/health` spans should be gone:

```traceql
{ resource.service.name = "frontend" && kind = server && span.url.path =~ ".*(css|js|ico|png|health)" }
```

Paste into Grafana → **Explore** → **Tempo**; the query should return no results.

**Part 2** — [Open in Grafana (Tempo)](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20resource.service.name%20%3D%20%5C%22frontend%5C%22%20%26%26%20span.enduser.id%20%21%3D%20nil%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D)

```traceql
{ resource.service.name = "frontend" && span.enduser.id != nil }
```

`enduser.id` should be an 8-character hex string. Expected hash for `alice`:

```text
SHA-256("alice") → 2bd806c9...  (first 8 chars: 2bd806c9)
```

**Part 3** — Tempo should now contain **only** error traces and slow traces, with nothing boring in between.

_Signal is present_ — errors and slow traces survived sampling — [Open in Grafana (Tempo)](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20status%20%3D%20error%20%7D%20%7C%7C%20%7B%20duration%20%3E%201s%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D):

```traceql
{ status = error } || { duration > 1s }
```

_Noise is gone_ — the interesting one — [Open in Grafana (Tempo)](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20kind%20%3D%20server%20%26%26%20status%20%3D%20ok%20%26%26%20duration%20%3C%20100ms%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D):

```traceql
{ kind = server && status = ok && duration < 100ms }
```

Fast successful server spans should **not** appear. A handful of hits are acceptable if the query window crosses the pre-sampling period, or if a fast span belongs to a trace that tail sampling kept for a different reason (a sibling span exceeded the latency threshold — the whole trace is kept together).

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

---

## Learn more

- [Transforming telemetry in the Collector](https://opentelemetry.io/docs/collector/transforming-telemetry/) — guide to the transform processor and OTTL
- [OTTL Playground](https://ottl.run/) — experiment with OTTL expressions interactively
- [Tail Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.149.0/processor/tailsamplingprocessor) — every policy type and tuning knob
- [OTel sampling](https://opentelemetry.io/docs/concepts/sampling/) — head vs tail, tradeoffs, and when to use each
- [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) — source of the `filter`, `transform`, and `tail_sampling` processors

---

## Catch up

```bash
git checkout origin/05-processing
```

---

[← Exercise 04](04-customizing-instrumentations.md) | [Exercise 06 →](06-manual-instrumentation.md)
