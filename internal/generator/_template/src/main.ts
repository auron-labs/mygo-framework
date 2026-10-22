import { createForm, type FormState } from "./framework/form";
import { runRequest } from "./framework/request";
import { Notes, events, type CreateNoteInput, type CreateNoteOutput, type Note } from "./mygo";

const formElement = document.querySelector<HTMLFormElement>("#note-form")!;
const input = document.querySelector<HTMLInputElement>("#note-body")!;
const submit = formElement.querySelector<HTMLButtonElement>("button")!;
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

const noteForm = createForm<CreateNoteInput, CreateNoteOutput>({
  initialValues: { body: "" },
  submit: (values) => runRequest(() => Notes.create({ body: values.body })),
});

const renderForm = (state: FormState<CreateNoteInput>) => {
  if (input.value !== state.values.body) input.value = state.values.body;
  const fieldError = state.fieldErrors.body;
  bodyError.textContent = fieldError ?? "";
  bodyError.hidden = fieldError === undefined;
  if (fieldError) input.setAttribute("aria-invalid", "true");
  else input.removeAttribute("aria-invalid");

  formError.textContent = state.formError ?? "";
  formError.hidden = state.formError === null || !ready;
  input.disabled = !ready || state.submitting;
  submit.disabled = !ready || state.submitting;
  submit.textContent = state.submitting ? "Adding…" : "Add note";
};

const showStarting = () => {
  ready = false;
  runtimeState.hidden = false;
  runtimeState.classList.remove("runtime-state--failed");
  runtimeState.setAttribute("role", "status");
  runtimeHeading.textContent = "Starting notes…";
  runtimeDetail.textContent = "Opening your local notes and checking the database.";
  collection.setAttribute("aria-busy", "true");
  renderForm(noteForm.getState());
};

const showReady = () => {
  ready = true;
  runtimeState.hidden = true;
  collection.setAttribute("aria-busy", "false");
  renderForm(noteForm.getState());
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
  renderForm(noteForm.getState());
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

const loadNotes = async () => {
  const result = await runRequest(() => Notes.list({}));
  if (!result.ok) {
    showRuntimeFailure(result.problem.message);
    return false;
  }

  renderNotes(result.data.notes);
  showReady();
  return true;
};

noteForm.subscribe(renderForm);
const stopNoteChanges = events.notesChanged.on(() => void loadNotes());

formElement.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!ready) return;

  status.textContent = "";
  const result = await noteForm.submit();
  if (!result) return;
  if (!result.ok) {
    if (result.problem.code === "unavailable") showRuntimeFailure(result.problem.message);
    input.focus();
    return;
  }

  noteForm.reset();
	status.textContent = "Note added.";
	input.focus();
});

input.addEventListener("input", () => {
  noteForm.setField("body", input.value);
});

window.addEventListener("pagehide", () => {
	noteForm.dispose();
	stopNoteChanges();
}, { once: true });

showStarting();
void loadNotes();
