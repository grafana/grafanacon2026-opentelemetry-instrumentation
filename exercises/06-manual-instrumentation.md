# Exercise 06 — Manual Instrumentation

[← Exercise 05](05-processing.md)

In this exercise you write OpenTelemetry instrumentation by hand — adding DB tracing to the
backend and a login instrumentation shell on the frontend.

Unlike the earlier exercises, this one touches ~17 files (including a new ~250-line Go file)
and is meant to be read and **explored**, not typed out. The doc walks through the main
design decisions; the full diff lives on GitHub.

> [!TIP]
> Open the [full diff for this exercise](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/compare/05-processing...06-manual-instrumentation)
> in a second tab and read it alongside this walkthrough. To run the finished version
> locally: `git checkout origin/06-manual-instrumentation`.

## Contents

- [The instrumentation as a wrapper pattern](#the-instrumentation-as-a-wrapper-pattern)
- [Part 1 — Backend DB (Go)](#part-1--backend-db-go)
- [Part 2 — Frontend Login (Node.js)](#part-2--frontend-login-nodejs)
- [Tests](#tests)
- [Verify](#verify)
- [Catch up](#catch-up)

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
> in the frontend case it wraps custom application auth flow.

---

## Part 1 — Backend DB (Go)

[**→ Browse Part 1 on GitHub**](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/compare/05-processing...06-manual-instrumentation#files_bucket) — filter to `backend/`.

A new [`backend/db/instrumented.go`](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/db/instrumented.go) wraps `*sql.DB` with OTel. `Connect()` in
[`db.go`](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/db/db.go) now returns the wrapper, so every handler and middleware that took a
`*sql.DB` needs to accept `*dbpkg.DB` instead (`go build ./...` lists them all).

The key pieces to look for in `instrumented.go`:

- **`DB` struct embeds `*sql.DB`** — methods we don't override (e.g. `PingContext`) pass
  through unchanged. We only wrap the three that execute queries: `QueryContext`,
  `QueryRowContext`, and `ExecContext`.

- **Span attributes follow [DB semantic conventions](https://opentelemetry.io/docs/specs/semconv/database/database-spans/).**
  The span name and `db.query.summary` are a low-cardinality summary like `SELECT restaurants`
  (computed by `querySummary` from the first word of the SQL and the table name). The full
  SQL goes into `db.query.text`. `server.address`, `server.port`, and `db.namespace` are
  parsed from the DSN once at startup and reused for every span.

- **The `*Row` wrapper is the non-obvious bit.** `*sql.Row` only reveals its error when
  the caller calls `Scan` or `Err` — so if we end the span inside `QueryRowContext`, every
  single-row query always ends with status OK, even on real errors. The fix: return a
  custom `*Row` type that ends the span inside _its_ `Scan`/`Err` methods, guarded by
  `sync.Once` so it ends exactly once.

- **Errors are emitted as logs, not span events.** `endSpan` calls `slog.ErrorContext`,
  which (via the slog→OTel bridge in [`backend/telemetry.go`](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/backend/telemetry.go))
  emits a log correlated to the active span. This follows the [exceptions-as-logs spec](https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/).
  `sql.ErrNoRows` is treated as a normal outcome and does not set error status.

- **Metric attributes get `error.type` on failure.** The `db.client.operation.duration`
  histogram is recorded on every call; on error we append `error.type` so failures show
  up as a distinct dimension on the same metric.

---

## Part 2 — Frontend Login (Node.js)

[**→ Browse Part 2 on GitHub**](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/compare/05-processing...06-manual-instrumentation#files_bucket) — filter to `frontend/`.

A new [`frontend/otel-auth.js`](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/frontend/otel-auth.js) exports `instrumentLogin(provider, fn)`.
[`server.js`](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/frontend/server.js) uses it to wrap the OAuth callback, and records the
`auth.client.login.duration` metric directly in the plain-username login handler.

What to look for:

- **`instrumentLogin` takes a pure async callback.** `fn` does the actual login work
  and returns `{ outcome, user?, isNewUser? }` — it knows nothing about OTel. The
  wrapper starts a span, maps `outcome` to attributes and span status, records the
  histogram, and ends the span in `finally` so it always ends exactly once — even on
  a thrown exception.

- **Outcome-based error signalling.** Instead of `throw`-ing for business failures
  (`user_not_found`, `state_mismatch`), the callback returns them as `outcome` strings.
  The wrapper copies that onto `error.type` and sets span status ERROR. Thrown errors
  (e.g. network failure) are a separate path — `error.type` becomes the exception
  class name.

- **Plain-username login has no span, only a metric.** It is a single backend
  round-trip, not a multi-step flow, so starting a span adds noise without insight.
  The `loginDuration` histogram is exported from `otel-auth.js` and recorded directly
  in the `POST /login` handler with `auth.provider.name = "local"`.

- **`enduser.id` and `enduser.pseudo.id`** are set on the login span when the user
  is known, so you can search traces by user after the fact.

---

## Tests

Both parts use **in-memory exporters** for fast, hermetic unit tests — no real trace
backend required.

**Backend** — [tests/backend/instrumentation_test.go](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/tests/backend/instrumentation_test.go) uses an in-memory span recorder against a real (ephemeral) database.

**Frontend** — [tests/frontend/otel-auth.test.js](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/06-manual-instrumentation/tests/frontend/otel-auth.test.js) uses in-memory span and metric exporters.

```bash
make test
```

---

## Verify

```bash
docker compose up --build
```

> [!NOTE]
> Traces and metrics may take up to a minute to appear after the services start. If panels are empty or spans are missing, wait a moment and refresh.

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

## Learn more

- [Instrumentation concepts](https://opentelemetry.io/docs/concepts/instrumentation/) — when to reach for manual instrumentation vs. libraries
- [Go language guide](https://opentelemetry.io/docs/languages/go/) — tracer, meter, and logger APIs used throughout Part 1
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) — including [database](https://opentelemetry.io/docs/specs/semconv/database/) attributes (`db.query.summary`, `db.query.text`, `db.system.name`, …)

---

## Catch up

```bash
git checkout origin/06-manual-instrumentation
```

---

[← Exercise 05](05-processing.md)
