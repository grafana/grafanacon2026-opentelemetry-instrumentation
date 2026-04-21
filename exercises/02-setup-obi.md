# Exercise 02 — Setup OBI (OTel eBPF Instrumentation)

[← Exercise 01](01-setup-infra-metrics.md) | [Exercise 03 →](03-instrumenting-applications.md)

In this exercise you add [OBI](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation) — the OpenTelemetry eBPF Instrument — to the stack. OBI automatically captures HTTP and RPC metrics for any process on the host using Linux eBPF, with no code changes or language agents required.

> [!NOTE]
> OBI uses Linux eBPF and runs as a privileged Docker container. On macOS and Windows, Docker Desktop provides a Linux VM where OBI runs — all other containers share the same Linux kernel, so OBI can observe them with no extra setup. OBI cannot run directly on macOS or Windows (outside Docker).

<!-- -->

> [!NOTE]
> Using Podman? OBI needs rootful mode — see [PODMAN.md](../PODMAN.md#rootful-mode-for-exercise-02) before running this exercise.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Deploy OBI](#part-1--deploy-obi)
  - [Step 1 — Add the OBI service to docker-compose](#step-1--add-the-obi-service-to-docker-compose)
  - [Step 2 — Create the OBI config](#step-2--create-the-obi-config)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| File                                                                                                                                                                 | Changes                                                                                   |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/docker-compose.yaml)                                 | Add the `obi` service                                                                     |
| [obi/obi-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/obi/obi-config.yaml)                                 | New OBI config — targets the app containers and exports metrics to the collector          |
| [grafana/dashboards/red-metrics.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/grafana/dashboards/red-metrics.json) | Pre-provisioned RED metrics dashboard — request rate, error rate, and latency per service |

---

## Part 1 — Deploy OBI

### Step 1 — Add the OBI service to docker-compose

OBI needs to run as a privileged container with `pid: host` so it can observe all processes on the host. It also needs access to the Docker socket to attach container metadata to metrics.

In [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/docker-compose.yaml):

```diff
# docker-compose.yaml
   otel-collector:
     ...

+  obi:
+    container_name: obi
+    image: otel/ebpf-instrument:v0.6.0
+    # we can narrow the permissions with Linux capabilities
+    # giving full privileges for the sake of simplicity
+    privileged: true
+    # important so OBI can inspect other processes in the host
+    pid: host
+    volumes:
+      # required if you want extra container metadata attributes
+      - /var/run/docker.sock:/var/run/docker.sock:ro
+      - ./obi/obi-config.yaml:/etc/obi/config.yaml:ro
+    environment:
+      OTEL_EBPF_CONFIG_PATH: /etc/obi/config.yaml
+    depends_on:
+      - otel-collector
+
 volumes:
   db_data:
```

### Step 2 — Create the OBI config

Create [obi/obi-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/obi/obi-config.yaml). The `discovery.instrument` list scopes OBI to only the app containers — without it OBI would instrument every process on the host, including the collector itself.

```yaml
# obi/obi-config.yaml
otel_metrics_export:
  protocol: http/protobuf
  endpoint: http://otel-collector:4318
  interval: 5s
discovery:
  instrument:
    # to avoid OBI instrumenting ALL the processes in the host
    # (even the OTEL collector or the Docker services), we
    # explicitly enumerate here the containers of our tapas application
    - container_name: backend
    - container_name: frontend
    # - container_name: db
    # - container_name: lgtm
    # - container_name: otel-collector
```

---

## Verify

```bash
docker compose up --build
```

A pre-built RED metrics dashboard lives in [grafana/dashboards/red-metrics.json](../grafana/dashboards/red-metrics.json). It is automatically provisioned on startup.

> [!NOTE]
> Metrics may take up to a minute to appear after the services start. If the panels are empty, wait a moment and refresh.

Open <http://localhost:3000/d/red-metrics>. You should see request rate, error rate, and P95 latency panels for the `backend` and `frontend` services.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/) — a great tool to see what metrics are available.

---

## Learn more

- [OBI configuration reference](https://opentelemetry.io/docs/zero-code/obi/configure/) — full list of OBI config options
- [OpenTelemetry eBPF Instrumentation (OBI)](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation) — source repo and release notes

---

## Catch up

To skip ahead to the completed state of this exercise, check out the exercise branch:

```bash
git checkout origin/02-setup-obi
```

---

[← Exercise 01](01-setup-infra-metrics.md) | [Exercise 03 →](03-instrumenting-applications.md)
