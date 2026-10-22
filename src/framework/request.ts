import type { Problem } from "../mygo";

export interface WireResponse<T> {
  data: T | null;
  problem: Problem | null;
}

export type RequestResult<T> =
  | { ok: true; data: T }
  | { ok: false; problem: Problem };

const problem = (code: string, message: string): RequestResult<never> => ({
  ok: false,
  problem: { code, message, fieldErrors: [] },
});

const invalidResponse = () => problem("internal", "The application returned an invalid response.");
const transportFailure = () => problem("unavailable", "The application service could not be reached.");

// runRequest invokes a generated MyGo method once and unwraps its wire response.
export const runRequest = async <T>(invoke: () => Promise<WireResponse<T>>): Promise<RequestResult<T>> => {
  try {
    const response = await invoke();
    if (response.data !== null && response.problem === null) {
      return { ok: true, data: response.data };
    }
    if (response.data === null && response.problem !== null) {
      return { ok: false, problem: response.problem };
    }

    return invalidResponse();
  } catch {
    return transportFailure();
  }
};
