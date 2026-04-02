# Exercise 01 — Setup Infrastructure Metrics

[Exercise 02 →](02-setup-obi.md)

In this exercise you configure the OpenTelemetry Collector to scrape infrastructure metrics from the host and Docker and add a Grafana dashboard to visualize them.

## Contents

- [What you will change](#what-you-will-change)
- [Part 1 — Mount volumes into the collector](#part-1--mount-volumes-into-the-collector)
  - [Step 1 — Mount the Docker socket and host filesystem](#step-1--mount-the-docker-socket-and-host-filesystem)
- [Part 2 — Configure the collector](#part-2--configure-the-collector)
  - [Step 2 — Add receivers](#step-2--add-receivers)
  - [Step 3 — Add the resourcedetection processor](#step-3--add-the-resourcedetection-processor)
  - [Step 4 — Wire everything into the pipelines](#step-4--wire-everything-into-the-pipelines)
- [Part 3 — Add the Grafana dashboard](#part-3--add-the-grafana-dashboard)
  - [Step 5 — Add the Grafana dashboard](#step-5--add-the-grafana-dashboard)
- [Verify](#verify)
- [Catch up](#catch-up)

## What you will change

| Service   | File                                                                                                                                                                           | What changes                                                                                                |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| collector | [otel-collector/config.yaml](../otel-collector/config.yaml)                                                                                                                    | Add `hostmetrics` and `docker_stats` receivers; add `resourcedetection` processor; wire them into pipelines |
| collector | [docker-compose.yaml](../docker-compose.yaml)                                                                                                                                  | Mount the Docker socket and host filesystem into the collector container                                    |
| —         | [grafana/dashboards/hostmetrics.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/grafana/dashboards/hostmetrics.json) | New dashboard — CPU, memory, disk, network for the host and per-container CPU/memory                        |

---

## Part 1 — Mount volumes into the collector

### Step 1 — Mount the Docker socket and host filesystem

The `hostmetrics` receiver reads from the host filesystem and `docker_stats` reads from the Docker socket. Expose both to the collector container in [docker-compose.yaml](../docker-compose.yaml):

```diff
# docker-compose.yaml
   volumes:
     - ./otel-collector/config.yaml:/etc/otelcol-contrib/config.yaml:ro
+    - /var/run/docker.sock:/var/run/docker.sock:ro
+    - /:/hostfs:ro
```

> [!NOTE]
> **macOS:** Docker Desktop runs containers inside a Linux VM, so `/` here is the VM's root filesystem — not your Mac's. `hostmetrics` will report the VM's CPU, memory, and disk rather than your laptop's. The dashboard will populate and look correct, but the numbers reflect the VM. This is expected behavior on macOS.

---

## Part 2 — Configure the collector

### Step 2 — Add receivers

Add the following receivers to [otel-collector/config.yaml](../otel-collector/config.yaml).

#### [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/receiver/hostmetricsreceiver)

Scrapes CPU, disk, filesystem, memory, network, paging, and process metrics from the host every 10 s. `root_path` points to the bind-mounted host filesystem.

```yaml
hostmetrics:
  root_path: /hostfs
  collection_interval: 10s
  scrapers:
    cpu:
      metrics:
        system.cpu.logical.count:
          enabled: true
    disk: {}
    filesystem:
      metrics:
        system.filesystem.utilization:
          enabled: true
    load: {}
    memory:
      metrics:
        system.memory.utilization:
          enabled: true
        system.memory.limit:
          enabled: true
    network:
      metrics:
        system.network.connections:
          enabled: true
    paging: {}
    processes: {}
```

#### [docker_stats](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/receiver/dockerstatsreceiver)

Scrapes per-container CPU, memory, network, and block I/O metrics every 10 s via the Docker socket.

```yaml
docker_stats:
  endpoint: unix:///var/run/docker.sock
  collection_interval: 10s
  timeout: 5s
```

### Step 3 — Add the [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/processor/resourcedetectionprocessor) processor

Enriches every span, metric, and log with host/OS/container resource attributes detected at startup.

```yaml
processors:
  resourcedetection:
    detectors: [env, system, docker]
    timeout: 2s
    override: false
```

### Step 4 — Wire everything into the pipelines

```diff
 service:
   pipelines:
     traces:
       receivers: [otlp]
-      processors: [batch]
+      processors: [resourcedetection, batch]
       exporters: [otlphttp]
     metrics:
-      receivers: [otlp]
-      processors: [batch]
+      receivers: [otlp, docker_stats, hostmetrics]
+      processors: [resourcedetection, batch]
       exporters: [otlphttp]
     logs:
       receivers: [otlp]
-      processors: [batch]
+      processors: [resourcedetection, batch]
       exporters: [otlphttp]
```

---

## Part 3 — Add the Grafana dashboard

### Step 5 — Add the Grafana dashboard

A pre-built dashboard definition lives in [grafana/dashboards/hostmetrics.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/grafana/dashboards/hostmetrics.json). It is automatically provisioned on startup (no manual import needed).

```bash
# copies only this file from the exercise branch — does not switch branches
git checkout origin/01-setup-infra-metrics -- grafana/dashboards/hostmetrics.json
```

---

## Verify

```bash
docker compose up --build
make load  # runs continuously — keep it running in a separate terminal, Ctrl+C to stop
```

Open <http://localhost:3000/d/hostmetrics>. You should see CPU, memory, disk, and network panels populated within a few seconds.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/) — a great tool to see what metrics are available.

---

## Catch up

To skip ahead to the completed state of this exercise, check out the exercise branch:

```bash
git checkout 01-setup-infra-metrics
```

---

[Exercise 02 →](02-setup-obi.md)
