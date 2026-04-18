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

| File                                                                                                                                                                           | Changes                                                                                                     |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/otel-collector/config.yaml)                   | Add `hostmetrics` and `docker_stats` receivers; add `resourcedetection` processor; wire them into pipelines |
| [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/docker-compose.yaml)                                 | Mount the Docker socket and host filesystem into the collector container                                    |
| [grafana/dashboards/hostmetrics.json](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/grafana/dashboards/hostmetrics.json) | New dashboard — CPU, memory, disk, network for the host and per-container CPU/memory                        |

---

## Part 1 — Mount volumes into the collector

### Step 1 — Mount the Docker socket and host filesystem

The `hostmetrics` receiver reads from the host filesystem and `docker_stats` reads from the Docker socket. Expose both to the collector container in [docker-compose.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/docker-compose.yaml):

```diff
# docker-compose.yaml
   volumes:
     - ./otel-collector/config.yaml:/etc/otelcol-contrib/config.yaml:ro
+    - /var/run/docker.sock:/var/run/docker.sock:ro
+    - /:/hostfs:ro
```

> [!NOTE]
> **macOS/Windows:** Docker Desktop runs containers inside a Linux VM, so `/` here is the VM's root filesystem — not your host machine's. `hostmetrics` will report the VM's CPU, memory, and disk rather than your laptop's. The dashboard will populate and look correct, but the numbers reflect the VM. This is expected behavior on macOS and Windows.

---

## Part 2 — Configure the collector

### Step 2 — Add receivers

Add the following receivers to [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/otel-collector/config.yaml) under the existing `receivers:` key.

#### [docker_stats](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/receiver/dockerstatsreceiver)

Scrapes per-container CPU, memory, network, and block I/O metrics every 10 s via the Docker socket.

```diff
# otel-collector/config.yaml
 receivers:
   otlp:
     protocols:
       grpc:
         endpoint: 0.0.0.0:4317
       http:
         endpoint: 0.0.0.0:4318
+  docker_stats:
+    endpoint: unix:///var/run/docker.sock
+    collection_interval: 10s
+    timeout: 5s
```

#### [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/receiver/hostmetricsreceiver)

Scrapes CPU, disk, filesystem, memory, network, paging, and process metrics from the host every 10 s. `root_path` points to the bind-mounted host filesystem.

```diff
# otel-collector/config.yaml
 receivers:
   ...
+  hostmetrics:
+    root_path: /hostfs
+    collection_interval: 10s
+    scrapers:
+      cpu:
+        metrics:
+          system.cpu.logical.count:
+            enabled: true
+      disk: {}
+      filesystem:
+        metrics:
+          system.filesystem.utilization:
+            enabled: true
+      load: {}
+      memory:
+        metrics:
+          system.memory.utilization:
+            enabled: true
+          system.memory.limit:
+            enabled: true
+      network:
+        metrics:
+          system.network.connections:
+            enabled: true
+      paging: {}
+      processes: {}
```

### Step 3 — Add the [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.147.0/processor/resourcedetectionprocessor) processor

Enriches every span, metric, and log with resource attributes detected at startup. `env` reads `OTEL_RESOURCE_ATTRIBUTES` from the collector's own environment — not from each application container. This makes it suitable for deployment-wide attributes like `deployment.environment.name` that apply uniformly to all telemetry. Configure per-service resources on each application independently. `system` detects `host.name`, `os.type`, and related attributes from the host — required by the host metrics dashboard.

Add the processor to [otel-collector/config.yaml](https://github.com/grafana/grafanacon2026-opentelemetry-instrumentation/blob/01-setup-infra-metrics/otel-collector/config.yaml):

```diff
# otel-collector/config.yaml
 processors:
+  resourcedetection:
+    detectors: [env, system]
+    timeout: 2s
+    override: false
+
   batch:
     timeout: 1s
```

### Step 4 — Wire everything into the pipelines

```diff
# otel-collector/config.yaml
 service:
   pipelines:
     traces:
       receivers: [otlp]
-      processors: [batch]
+      processors: [resourcedetection, batch]
       exporters: [otlp_http]
     metrics:
-      receivers: [otlp]
-      processors: [batch]
+      receivers: [otlp, docker_stats, hostmetrics]
+      processors: [resourcedetection, batch]
       exporters: [otlp_http]
     logs:
       receivers: [otlp]
-      processors: [batch]
+      processors: [resourcedetection, batch]
       exporters: [otlp_http]
```

---

## Part 3 — Add the Grafana dashboard

### Step 5 — Add the Grafana dashboard

A pre-built dashboard definition lives in [grafana/dashboards/hostmetrics.json](../grafana/dashboards/hostmetrics.json). It is automatically provisioned on startup (no manual import needed).

---

## Verify

```bash
docker compose up --build
```

Open <http://localhost:3000/d/hostmetrics>. You should see CPU, memory, disk, and network panels populated within a few seconds.

Check out the [metrics drilldown](http://localhost:3000/a/grafana-metricsdrilldown-app/) — a great tool to see what metrics are available.

---

## Learn more

- [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) — source of the `hostmetrics`, `docker_stats`, and `resourcedetection` components
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) — naming for host, container, and system resource attributes

---

## Catch up

To skip ahead to the completed state of this exercise, check out the exercise branch:

```bash
git checkout origin/01-setup-infra-metrics
```

---

[Exercise 02 →](02-setup-obi.md)
