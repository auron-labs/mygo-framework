import { Notes } from "./mygo";

const form = document.querySelector<HTMLFormElement>("#note-form")!;
const input = document.querySelector<HTMLInputElement>("#note-body")!;
const submit = form.querySelector<HTMLButtonElement>("button")!;
const error = document.querySelector<HTMLParagraphElement>("#form-error")!;
const empty = document.querySelector<HTMLParagraphElement>("#empty-state")!;
const list = document.querySelector<HTMLOListElement>("#note-list")!;
const count = document.querySelector<HTMLSpanElement>("#note-count")!;
const status = document.querySelector<HTMLParagraphElement>("#status")!;

const renderNotes = async () => {
  const notes = await Notes.list();

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

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  submit.disabled = true;
  submit.textContent = "Adding…";
  error.hidden = true;

  try {
    const response = await Notes.create(input.value);
    if (response.problem) {
      throw new Error(response.problem.message);
    }
    if (!response.data) {
      throw new Error("The note could not be added.");
    }
    input.value = "";
    await renderNotes();
    status.textContent = "Note added.";
    input.focus();
  } catch (cause) {
    error.textContent = cause instanceof Error ? cause.message : "The note could not be added.";
    error.hidden = false;
    input.focus();
  } finally {
    submit.disabled = false;
    submit.textContent = "Add note";
  }
});
renderNotes().catch((cause: unknown) => {
  error.textContent = cause instanceof Error ? cause.message : "The notes could not be loaded.";
  error.hidden = false;
});
