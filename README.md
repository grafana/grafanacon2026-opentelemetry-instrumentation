# Barcelona Tapas Finder — OpenTelemetry Workshop

A demo application for learning OpenTelemetry instrumentation. It helps users discover tapas restaurants in Barcelona.

## Architecture

- **Frontend**: Node.js/Express app with EJS templates (port 8080)
- **Backend**: Go REST API with gorilla/mux (port 8081)
- **Database**: PostgreSQL 16
- **OTel Collector**: Collects and forwards telemetry to LGTM
- **LGTM**: Grafana + Loki + Tempo + Mimir stack for observability (port 3000)

## Prerequisites

- Docker and Docker Compose
- Go (for backend tests)
- Node.js (for frontend tests)

## Running the Application

```bash
docker compose up --build
```

Then open <http://localhost:8080> in your browser.

## Running Tests

```bash
make test
```

This starts the database in Docker, runs the Go backend tests against it, and runs the Node.js frontend tests (using a mock backend).

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/restaurants` | List all restaurants |
| GET | `/api/restaurants/:id` | Get restaurant details |
| POST | `/api/restaurants` | Create restaurant (admin) |
| PUT | `/api/restaurants/:id` | Update restaurant (admin) |
| DELETE | `/api/restaurants/:id` | Delete restaurant (admin) |
| POST | `/api/restaurants/:id/photos` | Upload photo (admin) |
| GET | `/api/restaurants/:id/photos/:photo_id` | Get photo |
| POST | `/api/restaurants/:id/ratings` | Submit rating (auth) |
| GET | `/api/restaurants/:id/ratings` | List ratings |
| GET | `/api/users` | List users (admin) |
| GET | `/api/users/:id/favorites` | Get user favorites (auth) |

## Auth

Pass a `user-id` header with requests. Admin user IDs are defined in the database seed data.

## Chaos Mode

Set `CHAOS_MODE=true` in the `.env` file to enable intentional failures across both services: the backend will return a 500 on restaurant detail pages (bad SQL query) and fire N+1 photo queries on list pages through a single DB connection; the frontend will block the Node.js event loop on every search request, causing requests to queue up under concurrent load.

## Observability

The stack includes an OpenTelemetry Collector and a Grafana LGTM (Loki + Grafana + Tempo + Mimir) instance.

### OTel Collector

The collector ([otel-collector/config.yaml](otel-collector/config.yaml)) receives OTLP telemetry from the application and also scrapes infrastructure metrics:

- **hostmetrics**: CPU, disk, filesystem, memory, network, paging, and processes — collected every 30s from the host
- **docker_stats**: Per-container resource metrics collected every 10s via the Docker socket

All telemetry is forwarded to LGTM via OTLP HTTP.

### Grafana Dashboards

Open Grafana at <http://localhost:3000> (no login required).

- **Host Metrics dashboard**: <http://localhost:3000/d/hostmetrics-simple/host-metrics> — shows CPU, memory, disk, and network metrics for the host and CPU, memory metrics from containers

## Project Structure

```text
.
├── backend/          # Go REST API
├── db/               # Database init SQL
├── frontend/         # Node.js/Express frontend
├── tests/            # Integration tests
└── docker-compose.yml
```
