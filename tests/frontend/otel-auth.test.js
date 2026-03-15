"use strict";

/**
 * Tests for otel-auth.js instrumentation.
 *
 * Providers must be registered BEFORE requiring otel-auth.js, because
 * otel-auth.js calls trace.getTracer() and metrics.getMeter() at module load
 * time. Jest isolates module registries per test file, so this file gets a
 * fresh require cache and we can safely register providers here first.
 */

const {
  BasicTracerProvider,
  InMemorySpanExporter,
  SimpleSpanProcessor,
} = require("@opentelemetry/sdk-trace-base");
const {
  MeterProvider,
  PeriodicExportingMetricReader,
  InMemoryMetricExporter,
  AggregationTemporality,
} = require("@opentelemetry/sdk-metrics");
const { trace, metrics, SpanStatusCode } = require("@opentelemetry/api");

// ── Set up in-memory providers ────────────────────────────────────────────────

const spanExporter = new InMemorySpanExporter();
const tracerProvider = new BasicTracerProvider({
  spanProcessors: [new SimpleSpanProcessor(spanExporter)],
});
trace.setGlobalTracerProvider(tracerProvider);

const metricExporter = new InMemoryMetricExporter(
  AggregationTemporality.CUMULATIVE,
);
const meterProvider = new MeterProvider({
  readers: [
    new PeriodicExportingMetricReader({
      exporter: metricExporter,
      exportIntervalMillis: 1_000_000,
    }),
  ],
});
metrics.setGlobalMeterProvider(meterProvider);

// ── Load module under test (providers already registered above) ───────────────

const { instrumentLogin } = require("../../frontend/otel-auth");

// ── Fixtures ──────────────────────────────────────────────────────────────────

const ALICE = { id: "u1", username: "alice", is_admin: false };

// ── Lifecycle ─────────────────────────────────────────────────────────────────

afterEach(() => {
  spanExporter.reset();
  metricExporter.reset();
});

afterAll(async () => {
  await tracerProvider.shutdown();
  await meterProvider.shutdown();
});

// ── Helpers ───────────────────────────────────────────────────────────────────

async function getLoginDurationMetric() {
  await meterProvider.forceFlush();
  return metricExporter
    .getMetrics()
    .flatMap((rm) => rm.scopeMetrics)
    .flatMap((sm) => sm.metrics)
    .find((m) => m.descriptor.name === "auth.client.login.duration");
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("instrumentLogin", () => {
  test("emits a span with provider and user attributes", async () => {
    const fn = jest
      .fn()
      .mockResolvedValue({ outcome: "success", user: ALICE, isNewUser: false });

    await instrumentLogin("acme", fn);

    const spans = spanExporter.getFinishedSpans();
    expect(spans).toHaveLength(1);
    const span = spans[0];

    expect(span.name).toBe("login");
    expect(span.attributes["auth.operation.name"]).toBe("login");
    expect(span.attributes["auth.provider.name"]).toBe("acme");
    expect(span.attributes["error.type"]).toBeUndefined();
    expect(span.attributes["enduser.id"]).toBe("alice");
    expect(span.attributes["enduser.pseudo.id"]).toBe("u1");
    expect(span.status.code).toBe(SpanStatusCode.UNSET);
  });

  test("increments auth.client.new_users counter for first-time OAuth sign-in", async () => {
    const fn = jest
      .fn()
      .mockResolvedValue({ outcome: "success", user: ALICE, isNewUser: true });

    await instrumentLogin("acme", fn);

    await meterProvider.forceFlush();
    const newUsersMetric = metricExporter
      .getMetrics()
      .flatMap((rm) => rm.scopeMetrics)
      .flatMap((sm) => sm.metrics)
      .find((m) => m.descriptor.name === "auth.client.new_users");

    expect(newUsersMetric).toBeDefined();
    expect(newUsersMetric.dataPoints[0].attributes["auth.provider.name"]).toBe(
      "acme",
    );
    expect(newUsersMetric.dataPoints[0].value).toBe(1);
  });

  test("records auth.client.login.duration without error.type on success", async () => {
    const fn = jest
      .fn()
      .mockResolvedValue({ outcome: "success", user: ALICE, isNewUser: false });

    await instrumentLogin("acme", fn);

    const metric = await getLoginDurationMetric();
    expect(metric).toBeDefined();
    const dp = metric.dataPoints[0];
    expect(dp.attributes["auth.provider.name"]).toBe("acme");
    expect(dp.attributes["error.type"]).toBeUndefined();
  });

  test("sets error.type on span and metric when user is not found", async () => {
    const fn = jest.fn().mockResolvedValue({ outcome: "user_not_found" });

    await instrumentLogin("acme", fn);

    const span = spanExporter.getFinishedSpans()[0];
    expect(span.attributes["error.type"]).toBe("user_not_found");
    expect(span.status.code).toBe(SpanStatusCode.ERROR);
    expect(span.attributes["enduser.id"]).toBeUndefined();

    const metric = await getLoginDurationMetric();
    const dp = metric.dataPoints.find(
      (p) => p.attributes["error.type"] === "user_not_found",
    );
    expect(dp).toBeDefined();
  });
});
