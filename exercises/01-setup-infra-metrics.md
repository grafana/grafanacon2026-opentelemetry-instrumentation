# Exercise 01 — Setup Infrastructure Metrics

In this exercise you configure the OpenTelemetry Collector to scrape infrastructure metrics from the host and Docker and add a Grafana dashboard to visualise them.

## What you will change

| File | What changes |
|------|-------------|
| [otel-collector/config.yaml](../otel-collector/config.yaml) | Add `hostmetrics` and `docker_stats`; add `resourcedetection` processor; wire them into pipelines |
| [docker-compose.yml](../docker-compose.yml) | Mount the Docker socket and host filesystem into the collector container |
| [grafana/dashboards/hostmetrics.json](../grafana/dashboards/hostmetrics.json) | New dashboard — CPU, memory, disk, network for the host and per-container CPU/memory |

---

## Step 1 — Mount volumes into the collector

The `hostmetrics` receiver reads from the host filesystem and `docker_stats` reads from the Docker socket. Expose both to the collector container in [docker-compose.yml](../docker-compose.yml):

```diff
# docker-compose.yml
   volumes:
     - ./otel-collector/config.yaml:/etc/otelcol-contrib/config.yaml:ro
+    - /var/run/docker.sock:/var/run/docker.sock:ro
+    - /:/hostfs:ro
```

---

## Step 2 — Add receivers

Add the following receivers to [otel-collector/config.yaml](../otel-collector/config.yaml).

### [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/receiver/hostmetricsreceiver)

Scrapes CPU, disk, filesystem, memory, network, paging, and process metrics from the host every 30 s. `root_path` points to the bind-mounted host filesystem.

```yaml
hostmetrics:
  root_path: /hostfs
  collection_interval: 30s
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

### [docker_stats](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/receiver/dockerstatsreceiver)

Scrapes per-container CPU, memory, network, and block I/O metrics every 10 s via the Docker socket.

```yaml
docker_stats:
  endpoint: unix:///var/run/docker.sock
  collection_interval: 10s
  timeout: 5s
```

---

## Step 3 — Add the [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/processor/resourcedetectionprocessor) processor

Enriches every span, metric, and log with host/OS/container resource attributes detected at startup.

```yaml
processors:
  resourcedetection:
    detectors: [env, system, docker]
    timeout: 2s
    override: false
```

---

## Step 4 — Wire everything into the pipelines

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

## Step 5 — Add the Grafana dashboard

A pre-built dashboard definition lives in [grafana/dashboards/hostmetrics.json](../grafana/dashboards/hostmetrics.json). It is automatically provisioned on startup (no manual import needed).

```bash
git checkout 01-setup-infra-metrics -- grafana/dashboards/hostmetrics.json
```

---

## Verify

```bash
docker compose up --build
```

Open <http://localhost:3000/d/hostmetrics-simple/host-metrics>. You should see CPU, memory, disk, and network panels populated within a few seconds.

---

## Catch up

To skip ahead to the completed state of this exercise, check out the solution branch:

```bash
git checkout 01-setup-infra-metrics
```
