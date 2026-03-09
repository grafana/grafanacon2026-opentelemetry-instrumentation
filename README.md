# Barcelona Tapas Finder — OpenTelemetry Workshop

A demo application for learning OpenTelemetry instrumentation. It helps users discover tapas restaurants in Barcelona.

## Architecture

- **Frontend**: Node.js/Express app with EJS templates (port 3000)
- **Backend**: Go REST API with gorilla/mux (port 8080)
- **Database**: PostgreSQL 16

## Prerequisites

- Docker and Docker Compose
- Go (for backend tests)
- Node.js (for frontend tests)

## Running the Application

```bash
docker compose up --build
```

Then open <http://localhost:3000> in your browser.

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

## Project Structure

```text
.
├── backend/          # Go REST API
├── db/               # Database init SQL
├── frontend/         # Node.js/Express frontend
├── tests/            # Integration tests
└── docker-compose.yml
```
