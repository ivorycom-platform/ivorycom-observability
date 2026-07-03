import { afterEach, describe, expect, it, vi } from "vitest";
import { logs } from "@opentelemetry/api-logs";
import { bridgeConsole } from "../src/console-bridge";

describe("bridgeConsole", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("mirrors console calls to the OTel logs API and preserves stdout", () => {
    const emit = vi.fn();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit } as never);
    const stdout = vi.spyOn(console, "warn");

    bridgeConsole();
    console.warn("tenant %s over quota", "t-1");

    expect(stdout).toHaveBeenCalled();
    expect(emit).toHaveBeenCalledWith(
      expect.objectContaining({
        severityText: "warn",
        body: "tenant t-1 over quota",
      }),
    );
  });

  it("is idempotent — a second call must not double-wrap", () => {
    const emit = vi.fn();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit } as never);

    bridgeConsole();
    bridgeConsole();
    console.info("once");

    expect(emit).toHaveBeenCalledTimes(1);
  });
});
