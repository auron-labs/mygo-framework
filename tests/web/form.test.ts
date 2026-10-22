import { describe, expect, test } from "bun:test";
import { createForm } from "../../src/framework/form";
import type { RequestResult } from "../../src/framework/request";

interface Values {
  body: string;
  category: string;
}

interface Created {
  id: number;
}

describe("createForm", () => {
  test("tracks values, dirty fields, touched fields, and independent snapshots", () => {
    const form = createForm<Values, Created>({
      initialValues: { body: "", category: "general" },
      submit: async () => ({ ok: true, data: { id: 1 } }),
    });
    const observed: string[] = [];
    const unsubscribe = form.subscribe((state) => observed.push(state.values.body));

    expect(form.getState()).toEqual({
      values: { body: "", category: "general" },
      dirty: false,
      touched: {},
      submitting: false,
      fieldErrors: {},
      formError: null,
    });
    form.setField("body", "remember milk");
    expect(form.getState()).toMatchObject({
      values: { body: "remember milk", category: "general" },
      dirty: true,
      touched: { body: true },
    });
    form.setField("body", "");
    expect(form.getState().dirty).toBe(false);

    const snapshot = form.getState();
    (snapshot.values as Values).body = "mutated snapshot";
    expect(form.getState().values.body).toBe("");
    unsubscribe();
    form.setField("body", "after unsubscribe");
    expect(observed).toEqual(["", "remember milk", ""]);
  });

  test("resets state and accepts a new baseline", () => {
    const form = createForm<Values, Created>({
      initialValues: { body: "initial", category: "general" },
      submit: async () => ({ ok: true, data: { id: 1 } }),
    });

    form.setField("body", "changed");
    form.reset({ body: "saved", category: "work" });
    expect(form.getState()).toEqual({
      values: { body: "saved", category: "work" },
      dirty: false,
      touched: {},
      submitting: false,
      fieldErrors: {},
      formError: null,
    });
    form.setField("body", "other");
    form.reset();
    expect(form.getState().values).toEqual({ body: "saved", category: "work" });
  });

  test("prevents duplicate submission and clears pending state", async () => {
    let calls = 0;
    let complete!: (result: RequestResult<Created>) => void;
    const pending = new Promise<RequestResult<Created>>((resolve) => {
      complete = resolve;
    });
    const form = createForm<Values, Created>({
      initialValues: { body: "note", category: "general" },
      submit: () => {
        calls += 1;
        return pending;
      },
    });

    const first = form.submit();
    const duplicate = form.submit();
    expect(first).toBe(duplicate);
    await Promise.resolve();
    expect(calls).toBe(1);
    expect(form.getState().submitting).toBe(true);

    complete({ ok: true, data: { id: 7 } });
    await first;
    expect(form.getState().submitting).toBe(false);
  });

  test("maps known field errors and retains unmapped form context", async () => {
    const form = createForm<Values, Created>({
      initialValues: { body: "", category: "general" },
      submit: async () => ({
        ok: false,
        problem: {
          code: "validation",
          message: "Please correct the form.",
          fieldErrors: [
            { field: "body", code: "required", message: "is required" },
            { field: "serverOnly", code: "invalid", message: "is invalid" },
          ],
        },
      }),
    });

    await form.submit();
    expect(form.getState()).toMatchObject({
      submitting: false,
      fieldErrors: { body: "is required" },
      formError: "Please correct the form.",
    });
    form.setField("body", "fixed");
    expect(form.getState()).toMatchObject({ fieldErrors: {}, formError: null });
  });

  test("ignores obsolete completion after reset or disposal", async () => {
    let complete!: (result: RequestResult<Created>) => void;
    const pending = new Promise<RequestResult<Created>>((resolve) => {
      complete = resolve;
    });
    const form = createForm<Values, Created>({
      initialValues: { body: "note", category: "general" },
      submit: () => pending,
    });
    let notifications = 0;
    form.subscribe(() => notifications++);

    const submission = form.submit();
    form.reset();
    const notificationsAfterReset = notifications;
    complete({
      ok: false,
      problem: { code: "validation", message: "stale", fieldErrors: [] },
    });
    await submission;
    expect(form.getState()).toMatchObject({ submitting: false, formError: null });
    expect(notifications).toBe(notificationsAfterReset);

    form.dispose();
    form.setField("body", "ignored");
    expect(await form.submit()).toBeUndefined();
    expect(form.getState().values.body).toBe("note");
  });

  test("clears pending state when a submit callback throws", async () => {
    const form = createForm<Values, Created>({
      initialValues: { body: "note", category: "general" },
      submit: async () => {
        throw new Error("unexpected callback failure");
      },
    });

    const result = await form.submit();
    expect(result).toEqual({
      ok: false,
      problem: { code: "internal", message: "The form could not be submitted.", fieldErrors: [] },
    });
    expect(form.getState()).toMatchObject({
      submitting: false,
      fieldErrors: {},
      formError: "The form could not be submitted.",
    });
    expect(await form.submit()).toEqual(result);
  });
});
