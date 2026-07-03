import { format } from "node:util";
import { logs, SeverityNumber } from "@opentelemetry/api-logs";

/**
 * Mirror console.log/info/warn/error to the OTel logs pipeline so services
 * that log via console (the pollers) still reach Loki. Stdout behavior is
 * unchanged — Railway keeps capturing everything. Called by
 * initObservability(); safe to call once only (idempotent guard).
 */
let bridged = false;

export function bridgeConsole(): void {
  if (bridged) return;
  bridged = true;

  const methods = [
    ["log", SeverityNumber.INFO, "info"],
    ["info", SeverityNumber.INFO, "info"],
    ["warn", SeverityNumber.WARN, "warn"],
    ["error", SeverityNumber.ERROR, "error"],
  ] as const;

  for (const [method, severityNumber, severityText] of methods) {
    const original = console[method].bind(console);
    console[method] = (...args: unknown[]) => {
      original(...args);
      try {
        // Resolved per call: the global LoggerProvider may be (re)registered
        // after bridging, and getLogger on the API global is cheap.
        logs.getLogger("console").emit({
          severityNumber,
          severityText,
          body: format(...args),
        });
      } catch {
        // Never let telemetry break the app's logging path.
      }
    };
  }
}
