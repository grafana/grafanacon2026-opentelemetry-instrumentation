"use strict";

const { trace, metrics, SpanStatusCode } = require("@opentelemetry/api");

const SCHEMA_URL = "https://opentelemetry.io/schemas/1.40.0";
const tracer = trace.getTracer("tapas-auth", undefined, {
  schemaUrl: SCHEMA_URL,
});
const meter = metrics.getMeter("tapas-auth", undefined, {
  schemaUrl: SCHEMA_URL,
});

// Duration of login attempts. error.type is set on failures; absent on success.
const loginDuration = meter.createHistogram("auth.client.login.duration", {
  description: "Duration of login attempts",
  unit: "s",
});

// Counts new user accounts created via OAuth (returning users are not counted)
const newUserCounter = meter.createCounter("auth.client.new_users", {
  description: "New users registered via OAuth provider",
});

/**
 * Wraps an auth flow with a single span and metrics.
 *
 * `fn` is pure business logic — it knows nothing about OTel.
 * It should return `{ outcome, user?, isNewUser? }` or throw.
 *
 * outcome values: 'success' | 'user_not_found' | 'state_mismatch' | ...
 */
async function instrumentLogin(provider, fn) {
  const start = Date.now();
  return tracer.startActiveSpan("login", async (span) => {
    span.setAttribute("auth.operation.name", "login");
    span.setAttribute("auth.provider.name", provider);
    try {
      const result = await fn();
      if (result.outcome !== "success") {
        span.setAttribute("error.type", result.outcome);
        span.setStatus({ code: SpanStatusCode.ERROR });
      }
      if (result.user) {
        span.setAttribute("enduser.id", result.user.username);
        span.setAttribute("enduser.pseudo.id", String(result.user.id));
      }

      const attrs = { "auth.provider.name": provider };
      if (result.outcome !== "success") attrs["error.type"] = result.outcome;
      loginDuration.record((Date.now() - start) / 1000, attrs);
      if (result.isNewUser) {
        newUserCounter.add(1, { "auth.provider.name": provider });
      }
      return result;
    } catch (err) {
      span.setStatus({ code: SpanStatusCode.ERROR });
      loginDuration.record((Date.now() - start) / 1000, {
        "auth.provider.name": provider,
        "error.type": err.constructor?.name ?? "_OTHER",
      });
      throw err;
    } finally {
      span.end();
    }
  });
}

module.exports = { instrumentLogin, loginDuration };
