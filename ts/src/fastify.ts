import fp from "fastify-plugin";
import type { FastifyPluginAsync, FastifyRequest } from "fastify";
import { metrics } from "@opentelemetry/api";
import { logger } from "./logger";

// Per-request start time, stashed on the request object.
const START = Symbol("obsStart");

interface Timed extends FastifyRequest {
  [START]?: bigint;
}

/**
 * Fastify plugin: records the RED duration histogram
 * (http.server.request.duration, seconds) labelled by route/method/status, and
 * emits a standardized JSON access log per request. Most spans come from the
 * NodeSDK auto-instrumentation; this adds the metric + access log.
 */
const plugin: FastifyPluginAsync = async (app) => {
  const meter = metrics.getMeter("ivorycom/http");
  const dur = meter.createHistogram("http.server.request.duration", {
    unit: "s",
    description: "HTTP server request duration",
  });

  app.addHook("onRequest", async (req: Timed) => {
    req[START] = process.hrtime.bigint();
  });

  app.addHook("onResponse", async (req: Timed, reply) => {
    const start = req[START];
    const seconds = start ? Number(process.hrtime.bigint() - start) / 1e9 : 0;
    const route = req.routeOptions?.url ?? req.url;
    dur.record(seconds, {
      "http.route": route,
      "http.request.method": req.method,
      "http.response.status_code": String(reply.statusCode),
    });
    logger.info(
      { route, method: req.method, status: reply.statusCode, duration_s: seconds },
      "request",
    );
  });
};

export const fastifyObservability = fp(plugin, { name: "observability" });
