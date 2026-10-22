import { Notes, type Note, type Problem } from "./mygo";

const form = document.querySelector<HTMLFormElement>("#note-form")!;
const input = document.querySelector<HTMLInputElement>("#note-body")!;
const submit = form.querySelector<HTMLButtonElement>("button")!;
const bodyError = document.querySelector<HTMLParagraphElement>("#body-error")!;
const formError = document.querySelector<HTMLParagraphElement>("#form-error")!;
const empty = document.querySelector<HTMLParagraphElement>("#empty-state")!;
const list = document.querySelector<HTMLOListElement>("#note-list")!;
const collection = document.querySelector<HTMLElement>(".collection")!;
const count = document.querySelector<HTMLSpanElement>("#note-count")!;
const runtimeState = document.querySelector<HTMLElement>("#runtime-state")!;
const runtimeHeading = document.querySelector<HTMLHeadingElement>("#runtime-state-heading")!;
const runtimeDetail = document.querySelector<HTMLParagraphElement>("#runtime-state-detail")!;
const status = document.querySelector<HTMLParagraphElement>("#status")!;

let ready = false;

const setComposerEnabled = (enabled: boolean) => {
  input.disabled = !enabled;
  submit.disabled = !enabled;
};

const showStarting = () => {
  ready = false;
  runtimeState.hidden = false;
  runtimeState.classList.remove("runtime-state--failed");
  runtimeState.setAttribute("role", "status");
  runtimeHeading.textContent = "Starting notes…";
  runtimeDetail.textContent = "Opening your local notes and checking the database.";
  collection.setAttribute("aria-busy", "true");
  setComposerEnabled(false);
};

const showReady = () => {
  ready = true;
  runtimeState.hidden = true;
  collection.setAttribute("aria-busy", "false");
  setComposerEnabled(true);
};

const showRuntimeFailure = (message: string) => {
  ready = false;
  runtimeState.hidden = false;
  runtimeState.classList.add("runtime-state--failed");
  runtimeState.setAttribute("role", "alert");
  runtimeHeading.textContent = "Notes are unavailable";
  runtimeDetail.textContent = message;
  collection.setAttribute("aria-busy", "false");
  empty.hidden = true;
  setComposerEnabled(false);
};

const renderNotes = (notes: Note[]) => {
  list.replaceChildren(
    ...notes.map((note) => {
      const item = document.createElement("li");
      item.textContent = note.body;
      return item;
    }),
  );

  empty.hidden = notes.length > 0;
  count.textContent = String(notes.length);
  count.setAttribute("aria-label", `${notes.length} ${notes.length === 1 ? "note" : "notes"}`);
};

const clearFormErrors = () => {
  bodyError.hidden = true;
  bodyError.textContent = "";
  formError.hidden = true;
  formError.textContent = "";
  input.removeAttribute("aria-invalid");
};

const showFormProblem = (problem: Problem) => {
  const fieldError = problem.fieldErrors.find((field) => field.field === "body");
  if (fieldError) {
    bodyError.textContent = fieldError.message;
    bodyError.hidden = false;
    input.setAttribute("aria-invalid", "true");
  } else {
    formError.textContent = problem.message;
    formError.hidden = false;
  }
  input.focus();
};

const loadNotes = async () => {
  try {
    const response = await Notes.list({});
    if (response.problem) {
      showRuntimeFailure(response.problem.message);
      return false;
    }
    if (!response.data) {
      showRuntimeFailure("The notes could not be loaded.");
      return false;
    }

    renderNotes(response.data.notes);
    showReady();
    return true;
  } catch {
    showRuntimeFailure("The notes service could not be reached.");
    return false;
  }
};

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!ready) return;

  clearFormErrors();
  setComposerEnabled(false);
  submit.textContent = "Adding…";

  try {
    const response = await Notes.create({ body: input.value });
    if (response.problem) {
      if (response.problem.code === "unavailable") {
        showRuntimeFailure(response.problem.message);
      } else {
        showFormProblem(response.problem);
      }
      return;
    }
    if (!response.data) {
      formError.textContent = "The note could not be added.";
      formError.hidden = false;
      input.focus();
      return;
    }

    input.value = "";
    if (await loadNotes()) {
      status.textContent = "Note added.";
      input.focus();
    }
  } catch {
    formError.textContent = "The note could not be added.";
    formError.hidden = false;
    input.focus();
  } finally {
    submit.textContent = "Add note";
    if (ready) setComposerEnabled(true);
  }
});

input.addEventListener("input", () => {
  if (!bodyError.hidden) clearFormErrors();
});

showStarting();
void loadNotes();
