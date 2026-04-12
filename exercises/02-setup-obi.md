# Exercise 02 — Setup OBI (OTel eBPF Instrumentation)

[← Exercise 01](01-setup-infra-metrics.md) | [Exercise 03 →](03-instrumenting-applications.md)

In this exercise you add [OBI](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation) — the OpenTelemetry eBPF Instrument — to the stack. OBI automatically captures HTTP and RPC metrics for any process on the host using Linux eBPF, with no code changes or language agents required.

> [!NOTE]
> OBI uses Linux eBPF and requires a Linux host. On macOS, the `obi` container will fail to start — skip this exercise or use a Linux VM / remote Docker context.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Deploy OBI](#part-1--deploy-obi)
  - [Step 1 — Add the OBI service to docker-compose](#step-1--add-the-obi-service-to-docker-compose)
  - [Step 2 — Create the OBI config](#step-2--create-the-obi-config)
- [Part 2 — Add the Grafana dashboard](#part-2--add-the-grafana-dashboard)
  - [Step 3 — Add the Grafana dashboard](#step-3--add-the-grafana-dashboard)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| Service | File                                                                                                                                                                 | What changes                                                                     |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| —       | [docker-compose.yaml](../docker-compose.yaml)                                                                                                                        | Add the `obi` service                                                            |
| obi     | [obi/obi-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/obi/obi-config.yaml)                                 | New OBI config — targets the app containers and exports metrics to the collector |
| —       | [grafana/dashboards/red-metrics.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/grafana/dashboards/red-metrics.json) | New RED metrics dashboard — request rate, error rate, and latency per service    |

---

## Part 1 — Deploy OBI

TODO!!! in general, can we add line numbers to diff?
TODO!!! link to docker-compose

### Step 1 — Add the OBI service to docker-compose

OBI needs to run as a privileged container with `pid: host` so it can observe all processes on the host. It also needs access to the Docker socket to attach container metadata to metrics.

```diff
+  obi:
+    container_name: obi
+    image: otel/ebpf-instrument:main
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
```

### Step 2 — Create the OBI config

TODO!!! link to branch, not to main, confusing
Create [obi/obi-config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/obi/obi-config.yaml). The `discovery.instrument` list scopes OBI to only the app containers — without it OBI would instrument every process on the host, including the collector itself.

```yaml
otel_metrics_export:
  protocol: http/protobuf
  endpoint: http://otel-collector:4318
  interval: 5s
discovery:
  instrument:
    - container_name: db
    - container_name: backend
    - container_name: frontend
```

---

## Part 2 — Add the Grafana dashboard

### Step 3 — Add the Grafana dashboard

A pre-built RED metrics dashboard lives in [grafana/dashboards/red-metrics.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/02-setup-obi/grafana/dashboards/red-metrics.json). It is automatically provisioned on startup.

TODO!!! comment in slides on origin

```bash
# copies only this file from the exercise branch — does not switch branches
git checkout origin/02-setup-obi -- grafana/dashboards/red-metrics.json
```

---

## Verify

TODO!!! load gen is too noisy, don't start it right away because it hides errors in yaml

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

Open <http://localhost:3000/d/red-metrics>. You should see request rate, error rate, and P95 latency panels for the `backend` and `frontend` services.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/) — a great tool to see what metrics are available.

---

## Catch up

To skip ahead to the completed state of this exercise, check out the exercise branch:

```bash
git checkout origin/02-setup-obi
```

---

[← Exercise 01](01-setup-infra-metrics.md) | [Exercise 03 →](03-instrumenting-applications.md)
