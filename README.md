# ivorycom-observability

Shared **OpenTelemetry instrumentation** for Ivorycom backend services. One-line
init gives every service RED metrics, distributed traces, and trace-correlated
structured logs, all exported over OTLP to the central Grafana Alloy collector
([`ivorycom-observability-collector`](https://github.com/ivorycom-platform/ivorycom-observability-collector))
→ Grafana Cloud.

Polyglot repo (same shape as `ivorycom-identity`):

| Path | Artifact | Consumed as |
|------|----------|-------------|
| `go/` | Go module | `github.com/ivorycom-platform/ivorycom-observability/go` (tags `go/vX.Y.Z`) |
| `ts/` | Node package | `@ivorycom/observability` (git install) |

Python services vendor a small `otel_bootstrap.py` (only `document-parser` today).

## Environment contract (all languages)

Set per service (Railway variables):

```
OTEL_EXPORTER_OTLP_ENDPOINT = http://observability-collector.railway.internal:4318
OTEL_SERVICE_NAME           = <service>
DEPLOY_ENV                  = prod
```

Loki label discipline: labels are only `service`, `env`, `level`. `tenant_id`,
`trace_id`, `span_id` go in structured metadata, never labels.

## Go — Fiber + zerolog

```go
import obs "github.com/ivorycom-platform/ivorycom-observability/go"

shutdown, err := obs.Init(ctx, obs.Config{Service: "auth", Env: os.Getenv("DEPLOY_ENV")})
defer shutdown(context.Background())

// Register in order: otelfiber span first, then RED metrics.
for _, m := range obs.FiberMiddleware() { app.Use(m) }

log := obs.Logger(c.UserContext())   // zerolog: JSON stdout + OTLP, trace_id/span_id injected
client := obs.HTTPClient()           // outbound client that propagates W3C trace context
```

Add to a service:
```bash
go get github.com/ivorycom-platform/ivorycom-observability/go@go/v0.1.0
go mod tidy && [ -d vendor ] && go mod vendor
```

## Node — Fastify + pino

```ts
import { initObservability, logger, fastifyObservability } from "@ivorycom/observability";

await initObservability({ serviceName: "agent", env: process.env.DEPLOY_ENV ?? "prod" });
await app.register(fastifyObservability);   // RED metrics + access log
logger.info({ tenantId }, "handled request");
```

Install (git, like `@ivorycom/identity`):
```json
"@ivorycom/observability": "git+https://github.com/ivorycom-platform/ivorycom-observability.git#main"
```
For reliable auto-instrumentation, preload before app import:
```json
"start": "node -r @ivorycom/observability/ts/dist/instrumentation.js dist/server.js"
```

## Development

```bash
# Go
cd go && go test ./... && gofmt -l . && go vet ./... && staticcheck ./...

# Node
cd ts && npm install && npm run build && npm test && npm audit --omit=dev
```

Node runtime deps track the current OpenTelemetry JS line (experimental
`0.219.x`, stable `2.8.x`); `npm audit --omit=dev` is clean.
