import type { Problem } from "../mygo";
import type { RequestResult } from "./request";

export interface FormState<TValues extends object> {
  readonly values: Readonly<TValues>;
  readonly dirty: boolean;
  readonly touched: Readonly<Partial<Record<keyof TValues, boolean>>>;
  readonly submitting: boolean;
  readonly fieldErrors: Readonly<Record<string, string>>;
  readonly formError: string | null;
}

export interface FormOptions<TValues extends object, TData> {
  initialValues: TValues;
  submit(values: Readonly<TValues>): Promise<RequestResult<TData>>;
}

export interface FormController<TValues extends object, TData> {
  getState(): FormState<TValues>;
  setField<TKey extends keyof TValues>(field: TKey, value: TValues[TKey]): void;
  reset(values?: TValues): void;
  subscribe(listener: (state: FormState<TValues>) => void): () => void;
  submit(): Promise<RequestResult<TData> | undefined>;
  dispose(): void;
}

const unexpectedFailure = (): RequestResult<never> => ({
  ok: false,
  problem: { code: "internal", message: "The form could not be submitted.", fieldErrors: [] },
});

const copy = <TValues extends object>(values: TValues): TValues => ({ ...values });

export const createForm = <TValues extends object, TData>(
  options: FormOptions<TValues, TData>,
): FormController<TValues, TData> => {
  let initialValues = copy(options.initialValues);
  let values = copy(initialValues);
  let touched: Partial<Record<keyof TValues, boolean>> = {};
  let fieldErrors: Record<string, string> = {};
  let formError: string | null = null;
  let submitting = false;
  let disposed = false;
  let generation = 0;
  let active: { generation: number; promise: Promise<RequestResult<TData>> } | null = null;
  const dirtyFields = new Set<keyof TValues>();
  const listeners = new Set<(state: FormState<TValues>) => void>();

  const getState = (): FormState<TValues> => ({
    values: copy(values),
    dirty: dirtyFields.size > 0,
    touched: { ...touched },
    submitting,
    fieldErrors: { ...fieldErrors },
    formError,
  });

  const notify = () => {
    const state = getState();
    for (const listener of listeners) listener(state);
  };

  const clearErrors = () => {
    fieldErrors = {};
    formError = null;
  };

  const applyProblem = (problem: Problem) => {
    const nextFieldErrors: Record<string, string> = {};
    let hasUnmappedError = false;
    for (const error of problem.fieldErrors) {
      if (Object.hasOwn(values, error.field)) {
        nextFieldErrors[error.field] ??= error.message;
      } else {
        hasUnmappedError = true;
      }
    }

    fieldErrors = nextFieldErrors;
    formError = Object.keys(nextFieldErrors).length === 0 || hasUnmappedError ? problem.message : null;
  };

  const setField = <TKey extends keyof TValues>(field: TKey, value: TValues[TKey]) => {
    if (disposed) return;

    values = { ...values, [field]: value };
    touched = { ...touched, [field]: true };
    if (Object.is(value, initialValues[field])) dirtyFields.delete(field);
    else dirtyFields.add(field);

    const fieldName = String(field);
    if (Object.hasOwn(fieldErrors, fieldName)) {
      fieldErrors = { ...fieldErrors };
      delete fieldErrors[fieldName];
    }
    formError = null;
    notify();
  };

  const reset = (nextValues?: TValues) => {
    if (disposed) return;

    generation += 1;
    active = null;
    initialValues = copy(nextValues ?? initialValues);
    values = copy(initialValues);
    touched = {};
    dirtyFields.clear();
    submitting = false;
    clearErrors();
    notify();
  };

  const subscribe = (listener: (state: FormState<TValues>) => void) => {
    if (disposed) return () => undefined;

    listeners.add(listener);
    listener(getState());
    return () => listeners.delete(listener);
  };

  const submit = (): Promise<RequestResult<TData> | undefined> => {
    if (disposed) return Promise.resolve(undefined);
    if (active) return active.promise;

    submitting = true;
    clearErrors();
    const submissionGeneration = ++generation;
    const submittedValues = copy(values);
    notify();

    const promise = Promise.resolve()
      .then(() => options.submit(submittedValues))
      .catch((): RequestResult<TData> => unexpectedFailure())
      .then((result) => {
        if (!disposed && generation === submissionGeneration) {
          submitting = false;
          if (!result.ok) applyProblem(result.problem);
          notify();
        }
        if (active?.generation === submissionGeneration) active = null;

        return result;
      });

    active = { generation: submissionGeneration, promise };
    return promise;
  };

  const dispose = () => {
    if (disposed) return;

    disposed = true;
    generation += 1;
    active = null;
    submitting = false;
    listeners.clear();
  };

  return { getState, setField, reset, subscribe, submit, dispose };
};
