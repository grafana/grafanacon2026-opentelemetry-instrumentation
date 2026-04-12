# Barcelona Tapas Finder — OpenTelemetry Workshop

> [!IMPORTANT]
> You are on **[Exercise 01 — Setup infrastructure metrics](exercises/01-setup-infra-metrics.md)**

A demo application for learning OpenTelemetry instrumentation. It helps users discover tapas restaurants in Barcelona.

Workshop slides: [Getting started with OpenTelemetry instrumentation — GrafanaCON 2026](<Getting started with OpenTelemetry instrumentation - GrafanaCON 2026.pdf>)

## Table of Contents

- [Before the workshop](#before-the-workshop)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Workshop Structure](#workshop-structure)
- [Running the Application](#running-the-application)
- [Observability](#observability)
  - [OTel Collector](#otel-collector)
  - [Grafana Dashboards](#grafana-dashboards)
- [Technical Details](#technical-details)
  - [Project Structure](#project-structure)
  - [Running Tests](#running-tests)
  - [Load Generation](#load-generation)
  - [API Endpoints](#api-endpoints)
  - [Auth](#auth)
  - [Chaos Mode](#chaos-mode)
- [Useful Resources](#useful-resources)
  - [OpenTelemetry Fundamentals](#opentelemetry-fundamentals)
  - [Instrumentation & Components](#instrumentation--components)
  - [Customization & Processing](#customization--processing)
  - [Language-specific Guides](#language-specific-guides-used-in-this-workshop)

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
- Docker and Docker Compose — or Podman with the `docker compose` CLI, see [PODMAN.md](PODMAN.md) for setup

## Getting Started

Browse the app at `http://localhost:8080` — search restaurants, log in, submit ratings

Open Grafana at `http://localhost:3000` — the observability stack is configured and ready; we'll add instrumentation to populate it during the workshop. You should still be able to see OTel Collector self-diagnostic metrics in the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/)

## Workshop Structure

The workshop is divided into sections, each building on the previous one. Every section has a corresponding exercise branch you can check out if you get stuck or want to catch up.

| Exercise                                                                                            | Branch                            |
| --------------------------------------------------------------------------------------------------- | --------------------------------- |
| [01 — Setup infrastructure metrics in OpenTelemetry Collector](exercises/01-setup-infra-metrics.md) | `01-setup-infra-metrics`          |
| [02 — Setup eBPF instrumentation](exercises/02-setup-obi.md)                                        | `02-setup-obi`                    |
| [03 — Instrumenting applications](exercises/03-instrumenting-applications.md)                       | `03-instrumenting-applications`   |
| [04 — Customizing instrumentations](exercises/04-customizing-instrumentations.md)                   | `04-customizing-instrumentations` |
| [05 — Processing telemetry](exercises/05-processing.md)                                             | `05-processing`                   |
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
and forwards it to LGTM via OTLP HTTP and also scrapes infrastructure metrics:

- **hostmetrics**: CPU, disk, filesystem, load, memory, network, paging, and processes — collected every 10s from the host
- **docker_stats**: Per-container resource metrics collected every 10s via the Docker socket
- **resourcedetection** processor enriches every signal with host and env resource attributes

### Grafana Dashboards

Open Grafana at `http://localhost:3000` (no login required).

| Dashboard    | URL                                   | Description                                                                       |
| ------------ | ------------------------------------- | --------------------------------------------------------------------------------- |
| Host Metrics | <http://localhost:3000/d/hostmetrics> | CPU, memory, disk, and network metrics for the host; CPU and memory per container |

## Technical Details

### Project Structure

```text
.
├── backend/          # Go REST API
├── db/               # Database init SQL
├── frontend/         # Node.js/Express frontend
├── grafana/          # Grafana dashboard definitions and provisioning config
├── otel-collector/   # OpenTelemetry Collector config
├── tests/            # Integration tests
└── docker-compose.yaml
```

### Running Tests

Prerequisites: Go and Node.js

```bash
make test
```

This starts the database in Docker, runs the Go backend tests against it, and runs the Node.js frontend tests (using a mock backend).

### Load Generation

A [load script](load-test.js) runs as part of `docker compose up`, generating continuous traffic against the running application.

### API Endpoints

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
| DELETE | `/api/restaurants/:id/photos/:photo_id` | Delete photo (admin)      |
| POST   | `/api/restaurants/:id/ratings`          | Submit rating (auth)      |
| GET    | `/api/restaurants/:id/ratings`          | List ratings              |
| GET    | `/api/users`                            | List users (admin)        |
| POST   | `/api/users`                            | Create user               |
| GET    | `/api/users/by-username/:username`      | Look up user by username  |
| GET    | `/api/users/me/favorites`               | Get user favorites (auth) |

### Auth

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

### Chaos Mode

Set `CHAOS_MODE=true` in the `.env` file to enable intentional failures across both services: the backend will return a 500 on restaurant detail pages (bad SQL query) and fire N+1 photo queries on list pages through a single DB connection; the frontend will block the Node.js event loop on every search request, causing requests to queue up under concurrent load.

## Useful Resources

### OpenTelemetry Fundamentals

- **OpenTelemetry Official Site**: [opentelemetry.io](https://opentelemetry.io)
- **OTel Specifications**: [opentelemetry.io/docs/specs/otel](https://opentelemetry.io/docs/specs/otel/)
- **Semantic Conventions**: [opentelemetry.io/docs/specs/semconv](https://opentelemetry.io/docs/specs/semconv/)
- **W3C Trace Context**: [w3.org/TR/trace-context-2](https://www.w3.org/TR/trace-context-2/)
- **OTLP Protobuf Definitions**: [github.com/open-telemetry/opentelemetry-proto](https://github.com/open-telemetry/opentelemetry-proto)

### Instrumentation & Components

- **Instrumentation Concepts**: [opentelemetry.io/docs/concepts/instrumentation](https://opentelemetry.io/docs/concepts/instrumentation/)
- **OTel Collector Contrib**: [github.com/open-telemetry/opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib)
- **eBPF Instrumentation (OBI)**: [github.com/open-telemetry/opentelemetry-ebpf-instrumentation](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation)
- **Ecosystem Explorer**: [explorer.opentelemetry.io](https://explorer.opentelemetry.io)
- **OpenTelemetry Registry**: [opentelemetry.io/ecosystem/registry](https://opentelemetry.io/ecosystem/registry/)

### Customization & Processing

- **Transforming Telemetry (Collector)**: [opentelemetry.io/docs/collector/transforming-telemetry](https://opentelemetry.io/docs/collector/transforming-telemetry/)
- **OTTL Playground**: [ottl.run](https://ottl.run/)
- **Tail-based Sampling Concepts**: [opentelemetry.io/docs/concepts/sampling](https://opentelemetry.io/docs/concepts/sampling/#tail-sampling)

### Language-specific Guides (used in this workshop)

- **All languages**: [opentelemetry.io/docs/languages](https://opentelemetry.io/docs/languages/) — landing page with per-language SDK and API guides
- **Go**: [opentelemetry.io/docs/languages/go](https://opentelemetry.io/docs/languages/go/)
- **JavaScript (Node.js zero-code)**: [opentelemetry.io/docs/zero-code/js](https://opentelemetry.io/docs/zero-code/js/)
- **JavaScript sampling**: [opentelemetry.io/docs/languages/js/sampling](https://opentelemetry.io/docs/languages/js/sampling/)
