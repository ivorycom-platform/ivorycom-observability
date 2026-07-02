import pino from "pino";
import { trace, context } from "@opentelemetry/api";

// Single house logger for Node services: JSON to stdout, trace-correlated via a
// mixin. The OTLP log path is wired by initObservability(); logs also reach
// Loki through the OTel logs SDK once the NodeSDK is started.
export const logger = pino({
  level: process.env.LOG_LEVEL ?? "info",
  formatters: {
    level: (label) => ({ level: label }),
  },
  base: {
    service: process.env.OTEL_SERVICE_NAME,
    env: process.env.DEPLOY_ENV,
  },
  mixin() {
    const span = trace.getSpan(context.active());
    if (!span) return {};
    const sc = span.spanContext();
    return { trace_id: sc.traceId, span_id: sc.spanId };
  },
});
