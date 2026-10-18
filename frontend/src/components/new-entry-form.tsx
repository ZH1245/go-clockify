"use client";

import { useState } from "react";

import { api, type Project, type Task } from "@/lib/api";
import { nowForDateTimeInput } from "@/lib/time";

import { ProjectSelect } from "./project-select";

type Mode = "timer" | "manual";

type Props = {
  tasks: Task[];
  projects: Project[];
  run: (action: () => Promise<unknown>) => Promise<void>;
};

export function NewEntryForm({ tasks, projects, run }: Props) {
  const [title, setTitle] = useState("");
  const [mode, setMode] = useState<Mode>("timer");
  const [manualStart, setManualStart] = useState("");
  const [manualHours, setManualHours] = useState("");
  const [projectId, setProjectId] = useState<number | null>(null);

  const trimmed = title.trim();
  const hours = Number(manualHours);
  const canSubmit = trimmed !== "" && (mode === "timer" || (manualStart !== "" && hours > 0));

  // Entries are grouped by name: reuse the task with this title if there is one, otherwise create it.
  // The chosen project only applies when a new task is created; moving an existing task is done
  // from the task row (PUT /tasks/{id}/project) so this does not quietly rewrite it.
  async function findOrCreateTask(): Promise<Task> {
    const existing = tasks.find((task) => task.title.toLowerCase() === trimmed.toLowerCase());
    return existing ?? api.createTask(trimmed, projectId);
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    await run(async () => {
      const task = await findOrCreateTask();
      if (mode === "timer") {
        await api.startTimer(task.id);
      } else {
        await api.addManualEntry(task.id, new Date(manualStart).toISOString(), hours);
      }
    });
    setTitle("");
    setManualHours("");
  }

  function switchMode(next: Mode) {
    setMode(next);
    // Filled in on click, not on first render: Next.js pre-renders on the server, where "now" must not be read.
    if (next === "manual" && manualStart === "") setManualStart(nowForDateTimeInput());
  }

  return (
    <form
      onSubmit={submit}
      className="flex flex-col gap-3 rounded-xl border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900"
    >
      <div className="flex gap-2">
        <input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="What are you working on?"
          className="flex-1 rounded-lg border border-zinc-300 bg-transparent px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-zinc-700"
        />
        <div className="flex rounded-lg border border-zinc-300 p-0.5 text-sm dark:border-zinc-700">
          {(["timer", "manual"] as const).map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => switchMode(option)}
              aria-pressed={mode === option}
              className={`rounded-md px-3 py-1.5 ${
                mode === option ? "bg-zinc-200 font-medium dark:bg-zinc-700" : "text-zinc-500"
              }`}
            >
              {option === "timer" ? "Timer" : "Manual"}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 text-sm">
        <ProjectSelect
          projects={projects}
          value={projectId}
          onChange={setProjectId}
          label="New task project"
          className="rounded-md border border-zinc-300 bg-transparent px-2 py-1.5 dark:border-zinc-700"
        />

        <div className="ml-auto flex flex-wrap items-center gap-2">
          {mode === "manual" && (
            <>
              <input
                type="datetime-local"
                value={manualStart}
                onChange={(event) => setManualStart(event.target.value)}
                aria-label="When"
                className="rounded-md border border-zinc-300 bg-transparent px-2 py-1.5 dark:border-zinc-700"
              />
              <input
                type="number"
                min="0.25"
                max="24"
                step="0.25"
                value={manualHours}
                onChange={(event) => setManualHours(event.target.value)}
                placeholder="Hours"
                aria-label="Hours"
                className="w-24 rounded-md border border-zinc-300 bg-transparent px-2 py-1.5 dark:border-zinc-700"
              />
            </>
          )}
          <button
            type="submit"
            disabled={!canSubmit}
            className="rounded-lg bg-blue-600 px-5 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {mode === "timer" ? "Start" : "Add"}
          </button>
        </div>
      </div>
    </form>
  );
}
