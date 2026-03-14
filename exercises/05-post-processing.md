# Exercise 05 — Post-processing telemetry

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

| Service   | File                                                        | What changes                                               |
| --------- | ----------------------------------------------------------- | ---------------------------------------------------------- |
| collector | [otel-collector/config.yaml](../otel-collector/config.yaml) | Replace `enduser.id` with a short irreversible hash        |
| collector | [otel-collector/config.yaml](../otel-collector/config.yaml) | Rename custom log fields to stable HTTP semconv attributes |

---

## Part 1 — Anonymize `enduser.id`

[Exercise 04](04-customizing-instrumentations.md) sets `enduser.id` (username) on spans so traces are searchable by user. Usernames are personal data — a liability if traces are shipped to a third-party backend or retained long-term.

The [transform processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/processor/transformprocessor) rewrites attributes using [OTTL](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/pkg/ottl) expressions without touching application code.

> [!NOTE]
> Exercise 04 also sets `enduser.pseudo.id` — the internal numeric DB ID. It is already opaque, which is exactly what hashing `enduser.id` achieves. Both are still [linkable PII](https://opentelemetry.io/docs/specs/semconv/attributes-registry/enduser/). `enduser.pseudo.id` alone would have been sufficient — `enduser.id` was unnecessary to capture. The hashing technique below applies to any other sensitive string attribute.

Replace `enduser.id` with the first 8 hex characters of its SHA-256 digest.

### Step 1 — Add a transform processor for traces

In [otel-collector/config.yaml](../otel-collector/config.yaml):

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

In [otel-collector/config.yaml](../otel-collector/config.yaml):

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
make load
```

**Part 1** — log in as `alice`, browse the app, then query Tempo:

```traceql
{ resource.service.name = "frontend" && span.enduser.id != nil }
```

`enduser.id` should be an 8-character hex string. Expected hash for `alice`:

```text
SHA-256("alice") → 2bd806c9...  (first 8 chars: 2bd806c9)
```

**Part 2** — open Grafana → Explore → Loki:

```logql
{service_name=~"frontend|backend"} | http_request_method != ""
```

Request logs should carry `http.request.method`, `url.path`, and `http.response.status_code`.

---

## Catch up

```bash
git checkout 05-post-processing
```
