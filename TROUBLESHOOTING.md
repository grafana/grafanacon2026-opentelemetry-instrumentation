# Troubleshooting

## Startup is slow or appears hung

The first `docker compose up --build` downloads ~2 GB of images and compiles both services. This can take **5–10 minutes** on a slow connection. Subsequent builds are fast because layers are cached.

To check whether things are progressing:

```bash
docker compose logs -f
```

If services are healthy but Grafana shows no data, wait 30 seconds and reload — the first metrics batch takes a moment to arrive.

## Port conflicts

The app uses port **8080** (frontend) and **3000** (Grafana). If either is already in use:

```bash
# find what's using the port
lsof -i :8080
lsof -i :3000
```

Stop the conflicting process, then re-run `docker compose up --build`.

## Running backend tests manually

`make test-backend` sets the correct database URL automatically. If you run `go test` directly, set the URL explicitly:

```bash
cd tests/backend
TEST_DB_URL=postgres://postgres:postgres@localhost:5433/tapas?sslmode=disable go test ./...
```

Note: the test database runs on port **5433** (not 5432) to avoid conflicts with the running app stack.

## Collector config errors

Syntax errors in `otel-collector/config.yaml` cause the collector to exit silently. Check its logs:

```bash
docker compose logs otel-collector
```

Validate OTTL expressions by looking for `error_mode: ignore` — without it, a bad expression drops the entire telemetry item. You can also use the [OTTL playground](https://ottl.run/) to validate expressions.

## Podman

Setup is covered in [PODMAN.md](PODMAN.md). Common Podman-specific failures:

**Collector exits with `permission denied ... /var/run/docker.sock`** ([Exercise 01](exercises/01-setup-infra-metrics.md) and later) — Podman Machine's SELinux blocks container access to the Docker-compat socket. Fix per [PODMAN.md § Let containers read the Docker socket](PODMAN.md#macos--windows-let-containers-read-the-docker-socket).

**OBI exits with `memlock rlimit: operation not permitted`** ([Exercise 02](exercises/02-setup-obi.md)) — rootless Podman can't grant the eBPF capabilities OBI needs. Switch Podman Machine to rootful per [PODMAN.md § Rootful mode](PODMAN.md#rootful-mode-for-exercise-02).

**`/var/run/docker.sock: no such file or directory`** — the Docker-compat symlink is missing inside the Podman Machine VM (rare — Podman usually creates it). `podman machine ssh` into the VM and create it: `sudo ln -s /run/podman/podman.sock /var/run/docker.sock` (rootful) or `sudo ln -s /run/user/$UID/podman/podman.sock /var/run/docker.sock` (rootless).
