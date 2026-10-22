import { describe, expect, test } from "bun:test";
import type { Problem } from "../../src/mygo";
import { runRequest } from "../../src/framework/request";

describe("runRequest", () => {
  test("unwraps successful generated responses", async () => {
    let calls = 0;
    const result = await runRequest(async () => {
      calls += 1;
      return { data: { id: 7 }, problem: null };
    });

    expect(calls).toBe(1);
    expect(result).toEqual({ ok: true, data: { id: 7 } });
  });

  test.each([
    [false, false],
    [0, 0],
    ["", ""],
  ])("preserves the successful value %p", async (value, expected) => {
    const result = await runRequest(async () => ({ data: value, problem: null }));
    expect(result).toEqual({ ok: true, data: expected });
  });

  test("unwraps backend problems without changing their fields", async () => {
    const backendProblem: Problem = {
      code: "validation",
      message: "Please correct the highlighted fields.",
      fieldErrors: [{ field: "body", code: "required", message: "is required" }],
    };

    const result = await runRequest(async () => ({ data: null, problem: backendProblem }));
    expect(result).toEqual({ ok: false, problem: backendProblem });
  });

  test.each([
    { data: null, problem: null },
    {
      data: { id: 7 },
      problem: { code: "internal", message: "conflicting", fieldErrors: [] },
    },
  ])("normalizes an invalid wire response", async (response) => {
    const result = await runRequest(async () => response);
    expect(result).toEqual({
      ok: false,
      problem: {
        code: "internal",
        message: "The application returned an invalid response.",
        fieldErrors: [],
      },
    });
  });

  test.each([new Error("sensitive transport detail"), "bridge closed", null])(
    "normalizes an unexpected transport rejection",
    async (rejection) => {
      const result = await runRequest(async () => {
        throw rejection;
      });

      expect(result).toEqual({
        ok: false,
        problem: {
          code: "unavailable",
          message: "The application service could not be reached.",
          fieldErrors: [],
        },
      });
      expect(JSON.stringify(result)).not.toContain("sensitive transport detail");
    },
  );
});
