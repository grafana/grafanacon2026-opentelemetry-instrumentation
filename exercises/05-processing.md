# Exercise 05 — Processing telemetry

[← Exercise 04](04-customizing-instrumentations.md) | [Exercise 06 →](06-manual-instrumentation.md)

Use OTTL transform processors in the collector to anonymize sensitive span attributes and normalize log fields to semantic conventions.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Anonymize `enduser.id`](#part-1--anonymize-enduserid)
  - [Step 1 — Add a transform processor for traces](#step-1--add-a-transform-processor-for-traces)
  - [Step 2 — Wire the processor into the traces pipeline](#step-2--wire-the-processor-into-the-traces-pipeline)
- [Part 2 — Normalize HTTP log attributes](#part-2--normalize-http-log-attributes)
  - [Step 3 — Add a transform processor for logs](#step-3--add-a-transform-processor-for-logs)
  - [Step 4 — Wire the processor into the logs pipeline](#step-4--wire-the-processor-into-the-logs-pipeline)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| Service   | File                                                                                                                                                | What changes                                               |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| collector | [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml) | Replace `enduser.id` with a short irreversible hash        |
| collector | [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml) | Rename custom log fields to stable HTTP semconv attributes |

---

## Part 1 — Anonymize `enduser.id`

[Exercise 04](04-customizing-instrumentations.md) sets `enduser.id` (username) on spans so traces are searchable by user. Usernames are personal data — a liability if traces are shipped to a third-party backend or retained long-term.

The [transform processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/processor/transformprocessor) rewrites attributes using [OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/pkg/ottl) expressions without touching application code.

> [!NOTE]
> `enduser.id` is personal data. The hashing technique below applies to any sensitive string attribute you need to retain but anonymize — useful when you cannot change the application code.

Replace `enduser.id` with the first 8 hex characters of its SHA-256 digest.

### Step 1 — Add a transform processor for traces

In [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml):

```diff
+  transform/anonymize_enduser:
+    error_mode: ignore
+    trace_statements:
+      - context: span
+        statements:
+          - set(attributes["enduser.id"], Substring(SHA256(attributes["enduser.id"]), 0, 8)) where attributes["enduser.id"] != nil
```

`SHA256()` returns a 64-character hex digest; `Substring(str, start, length)` trims it to 8. The `where` guard skips spans without `enduser.id`. `error_mode: ignore` passes the span through unchanged if the expression fails.

### Step 2 — Wire the processor into the traces pipeline

```diff
     traces:
       receivers: [otlp]
-      processors: [resourcedetection, filter/drop_frontend_noise, batch]
+      processors: [resourcedetection, filter/drop_frontend_noise, transform/anonymize_enduser, batch]
       exporters: [otlphttp]
```

Place it after the filter — no point hashing attributes on spans that are about to be dropped.

---

## Part 2 — Normalize HTTP log attributes

Both services log requests with custom field names that don't match the [stable HTTP semantic conventions](https://opentelemetry.io/docs/specs/semconv/http/http-spans/):

| Emitted field | Stable semconv attribute    |
| ------------- | --------------------------- |
| `method`      | `http.request.method`       |
| `path`        | `url.path`                  |
| `status`      | `http.response.status_code` |

Correct names make dashboards and queries consistent across all services.

> [!TIP]
> Fix attribute names at the source when you can — it keeps the collector config simple and the change visible in code. Use the collector when the source is off-limits: a third-party library, another team's service, or a codebase you can't modify.

### Step 3 — Add a transform processor for logs

In [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/05-processing/otel-collector/config.yaml):

```diff
+  transform/normalize_log_http:
+    error_mode: ignore
+    log_statements:
+      - context: log
+        statements:
+          - set(attributes["http.request.method"], attributes["method"]) where attributes["method"] != nil
+          - delete_key(attributes, "method")
+          - set(attributes["url.path"], attributes["path"]) where attributes["path"] != nil
+          - delete_key(attributes, "path")
+          - set(attributes["http.response.status_code"], attributes["status"]) where attributes["status"] != nil
+          - delete_key(attributes, "status")
```

Each rename copies the value to the new key then deletes the old one. The `where` guard skips logs without the field (e.g. error or startup messages).

### Step 4 — Wire the processor into the logs pipeline

```diff
     logs:
       receivers: [otlp]
-      processors: [resourcedetection, batch]
+      processors: [resourcedetection, transform/normalize_log_http, batch]
       exporters: [otlphttp]
```

---

## Verify

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

**Part 1** — log in as `alice`, browse the app, then:

[Open in Grafana (Tempo)](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22tempo%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22traceql%22%2C%22query%22%3A%22%7B%20resource.service.name%20%3D%20%5C%22frontend%5C%22%20%26%26%20span.enduser.id%20%21%3D%20nil%20%7D%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D)

```traceql
{ resource.service.name = "frontend" && span.enduser.id != nil }
```

`enduser.id` should be an 8-character hex string. Expected hash for `alice`:

```text
SHA-256("alice") → 2bd806c9...  (first 8 chars: 2bd806c9)
```

**Part 2** — [Open in Grafana (Loki)](http://localhost:3000/explore?schemaVersion=1&orgId=1&panes=%7B%22abc%22%3A%7B%22datasource%22%3A%22loki%22%2C%22queries%22%3A%5B%7B%22refId%22%3A%22A%22%2C%22queryType%22%3A%22range%22%2C%22expr%22%3A%22%7Bservice_name%3D~%5C%22frontend%7Cbackend%5C%22%7D%20%7C%20http_request_method%20%21%3D%20%5C%22%5C%22%22%7D%5D%2C%22range%22%3A%7B%22from%22%3A%22now-1h%22%2C%22to%22%3A%22now%22%7D%7D%7D):

```logql
{service_name=~"frontend|backend"} | http_request_method != ""
```

Request logs should carry `http.request.method`, `url.path`, and `http.response.status_code`.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

---

## Catch up

```bash
git checkout origin/05-processing
```

---

[← Exercise 04](04-customizing-instrumentations.md) | [Exercise 06 →](06-manual-instrumentation.md)
