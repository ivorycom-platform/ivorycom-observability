// ESM preload entry — for "type": "module" services (contact-intelligence,
// pollers), where CommonJS `-r` preloads can't run before the app's imports.
//
// Usage (Nixpacks start script or Dockerfile CMD):
//   node --import @ivorycom/observability/register dist/server.js
//
// Registers the import-in-the-middle loader hook so ESM imports (pg, undici,
// redis, …) get patched — CommonJS-only require patching misses them — then
// starts the NodeSDK. Gated on OTEL_EXPORTER_OTLP_ENDPOINT: without it this
// file is a no-op, so local/dev/test runs never start the SDK.
import { register as registerLoaderHook, createRequire } from "node:module";

if (process.env.OTEL_EXPORTER_OTLP_ENDPOINT) {
  // exclude: packages whose module graphs carry import-order side effects and
  // break under IITM re-evaluation (openai's _shims registry throws "you must
  // import 'openai/shims/node' first"). None of them are instrumented anyway.
  registerLoaderHook("@opentelemetry/instrumentation/hook.mjs", {
    parentURL: import.meta.url,
    data: { exclude: [/openai/, /@anthropic-ai\/sdk/] },
  });
  const require = createRequire(import.meta.url);
  const { initObservability } = require("./ts/dist/index.js");
  await initObservability({
    serviceName: process.env.OTEL_SERVICE_NAME ?? "unknown",
    env: process.env.DEPLOY_ENV ?? "prod",
  });
}
