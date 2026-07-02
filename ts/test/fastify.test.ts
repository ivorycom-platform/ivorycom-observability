import { describe, it, expect } from "vitest";
import Fastify from "fastify";
import { fastifyObservability } from "../src/fastify";

describe("fastifyObservability", () => {
  it("registers and serves requests", async () => {
    const app = Fastify();
    await app.register(fastifyObservability);
    app.get("/ping", async () => ({ ok: true }));

    const res = await app.inject({ method: "GET", url: "/ping" });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ ok: true });

    await app.close();
  });

  it("records metrics without throwing on error responses", async () => {
    const app = Fastify();
    await app.register(fastifyObservability);
    app.get("/boom", async () => {
      throw new Error("boom");
    });

    const res = await app.inject({ method: "GET", url: "/boom" });
    expect(res.statusCode).toBe(500);

    await app.close();
  });
});
