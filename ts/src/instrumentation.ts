// Preload entry — the FIRST thing Node loads (via `node -r`), so the OTel
// auto-instrumentation can patch modules before the app requires them.
//
// Usage in a service:
//   "start": "node -r @ivorycom/observability/ts/dist/instrumentation dist/server.js"
// or copy a tiny wrapper that imports initObservability and runs it.
import { initObservability } from "./sdk";

void initObservability({
  serviceName: process.env.OTEL_SERVICE_NAME ?? "unknown",
  env: process.env.DEPLOY_ENV ?? "prod",
});
