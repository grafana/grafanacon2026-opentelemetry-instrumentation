# Barcelona Tapas Finder — OpenTelemetry Workshop

A demo application for learning OpenTelemetry instrumentation. It helps users discover tapas restaurants in Barcelona.

## Before the workshop

Pull all Docker images at home before the session to avoid slow startup on conference WiFi:

```bash
git clone https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation
cd grafanacon2026-opentelemetry-instrumentation
docker compose pull
docker compose up --build
```

The first build downloads images and compiles both services — it can take several minutes. Once images are cached, rebuilds are fast. See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) if you hit issues.

## Prerequisites

- Git
- Docker and Docker Compose

## Getting Started

Clone `grafana/grafanacon2026-opentelemetry-instrumentation`

```bash
git clone https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation
cd grafanacon2026-opentelemetry-instrumentation
```

Start all services

```bash
docker compose up --build
```

Browse the app at `http://localhost:8080` — search restaurants, log in, submit ratings

Open Grafana at `http://localhost:3000` — the observability stack is configured and ready; we'll add instrumentation to populate it during the workshop

## Workshop Structure

The workshop is divided into sections, each building on the previous one. Every section has a corresponding solution branch you can check out if you get stuck or want to catch up.

| Exercise                                                                                            | Branch                            |
| --------------------------------------------------------------------------------------------------- | --------------------------------- |
| [01 — Setup infrastructure metrics in OpenTelemetry Collector](exercises/01-setup-infra-metrics.md) | `01-setup-infra-metrics`          |
| [02 — Setup eBPF instrumentation](exercises/02-setup-obi.md)                                        | `02-setup-obi`                    |
| [03 — Instrumenting applications](exercises/03-instrumenting-applications.md)                       | `03-instrumenting-applications`   |
| [04 — Customizing instrumentations](exercises/04-customizing-instrumentations.md)                   | `04-customizing-instrumentations` |
| [05 — Post-processing telemetry](exercises/05-post-processing.md)                                   | `05-post-processing`              |
| [06 — Manual database instrumentation](exercises/06-manual-instrumentation.md)                      | `06-manual-instrumentation`       |

## Running the Application

```bash
docker compose up --build
```

Then open `http://localhost:8080` in your browser.

## Observability

The stack includes an OpenTelemetry Collector and a Grafana LGTM (Loki + Grafana + Tempo + Mimir) instance.

### OTel Collector

The collector ([otel-collector/config.yaml](otel-collector/config.yaml)) receives telemetry over OTLP
and forwards it to LGTM via OTLP HTTP.

Collector is configured and ready to receive telemetry.

### Grafana Dashboards

Open Grafana at `http://localhost:3000` (no login required).

We'll add dashboards throughout the workshop.

## Project Structure

```text
.
├── backend/          # Go REST API
├── db/               # Database init SQL
├── frontend/         # Node.js/Express frontend
├── grafana/          # Grafana dashboard definitions and provisioning config
├── otel-collector/   # OpenTelemetry Collector config
├── tests/            # Integration tests
└── docker-compose.yml
```

## Running Tests

### Test prerequisites

- Go
- Node.js

```bash
make test
```

This starts the database in Docker, runs the Go backend tests against it, and runs the Node.js frontend tests (using a mock backend).

## Load Generation

```bash
make load
```

Runs a [load script](load-test.js) that generates traffic against the running application.

## API Endpoints

| Method | Path                                    | Description               |
| ------ | --------------------------------------- | ------------------------- |
| GET    | `/api/health`                           | Health check              |
| GET    | `/api/restaurants`                      | List all restaurants      |
| GET    | `/api/restaurants/:id`                  | Get restaurant details    |
| POST   | `/api/restaurants`                      | Create restaurant (admin) |
| PUT    | `/api/restaurants/:id`                  | Update restaurant (admin) |
| DELETE | `/api/restaurants/:id`                  | Delete restaurant (admin) |
| POST   | `/api/restaurants/:id/photos`           | Upload photo (admin)      |
| GET    | `/api/restaurants/:id/photos/:photo_id` | Get photo                 |
| POST   | `/api/restaurants/:id/ratings`          | Submit rating (auth)      |
| GET    | `/api/restaurants/:id/ratings`          | List ratings              |
| GET    | `/api/users`                            | List users (admin)        |
| GET    | `/api/users/:id/favorites`              | Get user favorites (auth) |

## Auth

Two login methods are available:

- **Username login** — enter any pre-seeded username, no password required.
- **Acme SSO** — a simulated OAuth flow with a fake consent page.

Pre-seeded accounts:

| Username | Role  |
| -------- | ----- |
| `admin`  | admin |
| `alice`  | user  |
| `bob`    | user  |
| `carla`  | user  |

## Chaos Mode

Set `CHAOS_MODE=true` in the `.env` file to enable intentional failures across both services: the backend will return a 500 on restaurant detail pages (bad SQL query) and fire N+1 photo queries on list pages through a single DB connection; the frontend will block the Node.js event loop on every search request, causing requests to queue up under concurrent load.
