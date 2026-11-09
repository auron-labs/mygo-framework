import { afterEach, describe, expect, test } from "bun:test";
import * as Sentry from "@sentry/browser";
import { flushErrorReports, startErrorReporting } from "../../src/framework/errorReporting";

const servers: Array<{ stop: (force: boolean) => void }> = [];

const fakeEndpoint = () => {
  const bodies: string[] = [];
  const server = Bun.serve({
    port: 0,
    async fetch(request) {
      bodies.push(await request.text());
      return new Response("{}", { headers: { "access-control-allow-origin": "*" } });
    },
  });
  servers.push(server);
  return { dsn: `http://public@127.0.0.1:${server.port}/9`, bodies };
};

afterEach(async () => {
  await Sentry.close(0);
  servers.splice(0).forEach((server) => server.stop(true));
});

describe("startErrorReporting", () => {
  test("reports uncaught errors to the DSN from the Go side", async () => {
    const endpoint = fakeEndpoint();

    expect(await startErrorReporting(async () => endpoint.dsn)).toBe(true);
    (globalThis as { onerror?: (...args: unknown[]) => void }).onerror?.(
      "Uncaught Error: render exploded",
      "app.js",
      1,
      1,
      new Error("render exploded"),
    );
    await flushErrorReports();

    expect(endpoint.bodies.join("\n")).toContain("render exploded");
  });

  test("stays off and sends nothing without a DSN", async () => {
    const endpoint = fakeEndpoint();

    expect(await startErrorReporting(async () => "")).toBe(false);
    Sentry.captureException(new Error("ignored"));
    await flushErrorReports();

    expect(Sentry.getClient()).toBeUndefined();
    expect(endpoint.bodies).toEqual([]);
  });

  test("stays off when the DSN cannot be loaded", async () => {
    const reject = async (): Promise<string> => {
      throw new Error("service unavailable");
    };

    expect(await startErrorReporting(reject)).toBe(false);
    expect(Sentry.getClient()).toBeUndefined();
  });
});
