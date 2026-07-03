import { NodeSDK } from "@opentelemetry/sdk-node";
import { bridgeConsole } from "./console-bridge";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { BatchLogRecordProcessor } from "@opentelemetry/sdk-logs";
import { resourceFromAttributes } from "@opentelemetry/resources";
import {
  ATTR_SERVICE_NAME,
  ATTR_DEPLOYMENT_ENVIRONMENT_NAME,
} from "@opentelemetry/semantic-conventions";

export interface InitOptions {
  serviceName: string;
  env: string;
}

export interface Observability {
  shutdown: () => Promise<void>;
}

/**
 * Boot OpenTelemetry for a Node service: traces + metrics + logs exported over
 * OTLP/HTTP to the Alloy collector (OTEL_EXPORTER_OTLP_ENDPOINT), with HTTP,
 * Fastify, Postgres, undici/fetch and Redis auto-instrumented.
 *
 * Call this BEFORE the app's other imports — ideally via a `-r` preload — so
 * auto-instrumentation can patch modules before they are required.
 */
export async function initObservability(opts: InitOptions): Promise<Observability> {
  process.env.OTEL_SERVICE_NAME ??= opts.serviceName;
  process.env.DEPLOY_ENV ??= opts.env;

  const sdk = new NodeSDK({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: opts.serviceName,
      [ATTR_DEPLOYMENT_ENVIRONMENT_NAME]: opts.env,
    }),
    traceExporter: new OTLPTraceExporter(),
    metricReader: new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter(),
    }),
    logRecordProcessors: [new BatchLogRecordProcessor(new OTLPLogExporter())],
    instrumentations: [
      getNodeAutoInstrumentations({
        "@opentelemetry/instrumentation-fs": { enabled: false },
      }),
    ],
  });

  sdk.start();

  // Console → Loki: services that log via console (pollers) still get the
  // logs pillar. Must run after sdk.start() so the global LoggerProvider is
  // registered.
  bridgeConsole();

  return {
    shutdown: () => sdk.shutdown(),
  };
}
