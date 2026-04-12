# Exercise 06 — Manual Instrumentation

[← Exercise 05](05-processing.md)

In this exercise you write OpenTelemetry instrumentation by hand — adding DB tracing to the
backend and a login instrumentation shell on the frontend.

## Contents

- [What you will change](#what-you-will-change)
- [The instrumentation as a wrapper pattern](#the-instrumentation-as-a-wrapper-pattern)
- [Part 1 — Backend DB (Go)](#part-1--backend-db-go)
  - [Step 1 — Create instrumented.go](#step-1--create-backenddbinstrumentedgo)
  - [Step 2 — Update db.go](#step-2--update-backenddbdbgo)
- [Part 2 — Frontend Login (Node.js)](#part-2--frontend-login-nodejs)
  - [Step 3 — Create otel-auth.js](#step-3--create-frontendotel-authjs)
  - [Step 4 — Use instrumentLogin in server.js](#step-4--use-instrumentlogin-in-frontendserverjs)
- [Tests](#tests)
- [Verify](#verify)
- [Catch up](#catch-up)

---

## What you will change

| File                                                                                                                                                            | Lang | What changes                                                                   |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- | ------------------------------------------------------------------------------ |
| [backend/db/instrumented.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/db/instrumented.go) | Go   | New — `DB` wrapper with OTel instrumentation                                   |
| [backend/db/db.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/db/db.go)                     | Go   | Wrap `sql.DB` with `NewDB`                                                     |
| [frontend/otel-auth.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/frontend/otel-auth.js)           | JS   | New — `instrumentLogin` wrapper with OTel span and metrics                     |
| [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/frontend/server.js)                 | JS   | OAuth callback uses `instrumentLogin`; local login records the metric directly |

---

## The instrumentation as a wrapper pattern

Both parts follow the same principle: wrap business logic in an **instrumentation shell**
that starts a span, records metrics, handles errors, and ends the span — while the wrapped
code stays unaware of OTel.

In Go, `DB` is the wrapper — its methods call `startSpan`/`endSpan` around the real `sql.DB`
calls. In JS, `instrumentLogin` is the wrapper — it wraps an async callback `fn` that contains
pure login logic.

> [!IMPORTANT]
> Manual instrumentation is tedious and error-prone. Only do it when there is no high-quality
> library available, or when you have specific requirements a library cannot meet. In the backend
> case there is no library that follows OTel DB semantic conventions precisely enough;
> in the frontend case there is no library at all.

---

## Part 1 — Backend DB (Go)

### Step 1 — Create [backend/db/instrumented.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/db/instrumented.go)

**Starting a span** — use a low-cardinality summary (e.g. `SELECT restaurants`) as the span
name and `db.query.summary` attribute, and include the full SQL as `db.query.text`:

```go
func (db *DB) startSpan(ctx context.Context, query string) (context.Context, trace.Span, attribute.Set) {
    summary := querySummary(query)   // e.g. "SELECT restaurants"

    ctx, span := db.tracer.Start(ctx, summary,
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            semconv.DBQuerySummary(summary), // db.query.summary — low cardinality
            semconv.DBQueryText(query),      // db.query.text    — full SQL
            // plus server.address, server.port, db.namespace, db.system.name
        ),
    )
    // ...
}
```

**Ending a span** — set error status and emit a log via slog correlated with the span:

```go
func endSpan(ctx context.Context, span trace.Span, err error) {
    if err != nil && !errors.Is(err, sql.ErrNoRows) {
        slog.ErrorContext(ctx, "database error",
            slog.String("exception.type", fmt.Sprintf("%T", err)),
            slog.String("exception.message", err.Error()),
        )
        span.SetStatus(codes.Error, err.Error())
    }
    span.End()
}
```

The slog bridge in [backend/telemetry.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/telemetry.go) exports the log and
correlates it with the active trace via `ctx`.

Each method also records a `db.client.operation.duration` histogram with the same
`db.query.summary` and connection attributes.

---

### Step 2 — Update [backend/db/db.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/db/db.go)

Wrap the existing `sql.DB` connection with `NewDB` and update the return type:

```diff
+db, err := NewDB(conn, dsn)
+if err != nil {
+    return nil, fmt.Errorf("init db instrumentation: %w", err)
+}
-return conn, nil
+return db, nil

-func Connect() (*sql.DB, error) {
+func Connect() (*DB, error) {
```

---

## Part 2 — Frontend Login (Node.js)

### Step 3 — Create [frontend/otel-auth.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/frontend/otel-auth.js)

**Initialize tracer, meter, and instruments** once at module load:

```js
const SCHEMA_URL = "https://opentelemetry.io/schemas/1.40.0";
const tracer = trace.getTracer("tapas-auth", undefined, {
  schemaUrl: SCHEMA_URL,
});
const meter = metrics.getMeter("tapas-auth", undefined, {
  schemaUrl: SCHEMA_URL,
});

const loginDuration = meter.createHistogram("auth.client.login.duration", {
  description: "Duration of login attempts",
  unit: "s",
});

const newUserCounter = meter.createCounter("auth.client.new_users", {
  description: "New users registered via OAuth provider",
});
```

**The wrapper** — start a span, delegate to `fn`, handle outcomes and errors:

```js
async function instrumentLogin(provider, fn) {
  const start = Date.now();
  return tracer.startActiveSpan('login', async (span) => {
    // ...
    try {
      const result = await fn();
      if (result.outcome !== 'success') {
        span.setAttribute('error.type', result.outcome);
        span.setStatus({ code: SpanStatusCode.ERROR });
      }
      // ...
      loginDuration.record((Date.now() - start) / 1000, { 'auth.provider.name': provider, ... });
      return result;
    } catch (err) {
      span.setStatus({ code: SpanStatusCode.ERROR });
      loginDuration.record((Date.now() - start) / 1000, { ..., 'error.type': err.constructor?.name });
      throw err;
    } finally {
      span.end();
    }
  });
}
```

---

### Step 4 — Use `instrumentLogin` in [frontend/server.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/frontend/server.js)

The OAuth callback wraps all business logic inside `instrumentLogin`:

```js
app.post('/auth/acme/callback', async (req, res) => {
  const { username, state } = req.body;
  try {
    const result = await instrumentLogin('acme', async () => {
      // ... actual login logic
    });
  }
});
```

---

## Tests

Both parts use **in-memory exporters** for fast, hermetic unit tests — no real database
or trace backend required.

**Backend** — [tests/backend/instrumentation_test.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/tests/backend/instrumentation_test.go)
uses an in-memory span exporter against a real database.

**Frontend** — [tests/frontend/server.test.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/tests/frontend/server.test.js)
uses in-memory span and metric exporters.

```bash
make test
```

---

## Verify

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

Open Grafana at <http://localhost:3000/d/apm-dashboard>.

### Backend — DB spans

In the **DB outgoing calls** panel for the backend service you should see spans named
`SELECT restaurants`, `INSERT ratings`, `SELECT users`, etc. Click a span to verify:

- `db.system.name` = `postgresql`
- `db.query.summary` = `SELECT restaurants` (or similar)
- `db.query.text` = the full SQL string
- `server.address` = `db`
- `server.port` = `5432`
- `db.namespace` = `tapas`

For error spans (e.g. with Chaos Mode enabled), check the **Logs** panel — you should see
an error log with `exception.type` and `exception.message` correlated to the failing span.

### Frontend — login spans

Open the app at <http://localhost:8080>, log in via **Acme SSO** (OAuth), and then via the
plain username form. In Grafana, find a trace for the frontend service and locate the
`login` span. Verify:

- `auth.provider.name` = `acme`
- `enduser.id` = the username
- `enduser.pseudo.id` = the numeric user id as a string

The plain username login has no span — it only records the `auth.client.login.duration` metric.

Try logging in with a non-existent username via Acme SSO to trigger a `user_not_found` outcome.
The span should have `error.type` = `user_not_found` and span status `ERROR`. Check the
`auth.client.login.duration` metric in the **Metrics** panel — the `error.type` dimension
should appear on the histogram.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/), [traces drilldown](http://localhost:3000/a/grafana-exploretraces-app/), and [logs drilldown](http://localhost:3000/a/grafana-lokiexplore-app/) — great tools to see what telemetry is available.

---

## Catch up

```bash
git checkout origin/06-manual-instrumentation
```

---

[← Exercise 05](05-processing.md)
