"use client";

import { useState } from "react";

import { api, type Task, type TaskStatus } from "@/lib/api";
import {
  formatDateTime,
  formatDuration,
  formatTime,
  isRunning,
  itemSeconds,
  nowForDateTimeInput,
  taskSeconds,
} from "@/lib/time";

type Props = {
  task: Task;
  now: number;
  run: (action: () => Promise<unknown>) => Promise<void>;
};

const statusLabels: Record<TaskStatus, string> = {
  todo: "To do",
  in_progress: "In progress",
  done: "Done",
};

export function TaskRow({ task, now, run }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [manualStart, setManualStart] = useState(nowForDateTimeInput);
  const [manualHours, setManualHours] = useState("");

  const runningItem = task.items.find(isRunning);

  async function addManualEntry(event: React.FormEvent) {
    event.preventDefault();
    const hours = Number(manualHours);
    if (!manualStart || !(hours > 0)) return;
    await run(() => api.addManualEntry(task.id, new Date(manualStart).toISOString(), hours));
    setManualHours("");
  }

  return (
    <li className="rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div className="flex items-center gap-3 px-3 py-3">
        <button
          onClick={() => setExpanded(!expanded)}
          className="flex min-w-0 flex-1 items-center gap-3 text-left"
          aria-expanded={expanded}
        >
          <span
            className="flex h-7 min-w-7 items-center justify-center rounded-full bg-zinc-100 px-2 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
            title={`${task.timer_count} time entries`}
          >
            {task.timer_count}
          </span>
          <span className="truncate font-medium">{task.title}</span>
          <span className="text-zinc-400">{expanded ? "▾" : "▸"}</span>
        </button>

        <select
          value={task.status}
          onChange={(event) => void run(() => api.updateTaskStatus(task.id, event.target.value as TaskStatus))}
          className="rounded-md border border-zinc-200 bg-transparent px-2 py-1 text-xs dark:border-zinc-700"
        >
          {Object.entries(statusLabels).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>

        <span className={`w-20 text-right font-mono text-sm ${runningItem ? "text-blue-600" : ""}`}>
          {formatDuration(taskSeconds(task, now))}
        </span>

        {runningItem ? (
          <button
            onClick={() => void run(() => api.stopTimer(runningItem.id))}
            className="w-16 rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700"
          >
            Stop
          </button>
        ) : (
          <button
            onClick={() => void run(() => api.startTimer(task.id))}
            className="w-16 rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
          >
            Start
          </button>
        )}

        <button
          onClick={() => {
            if (confirm(`Delete "${task.title}" and all its time entries?`)) {
              void run(() => api.deleteTask(task.id));
            }
          }}
          className="text-zinc-400 hover:text-red-600"
          aria-label={`Delete ${task.title}`}
        >
          ✕
        </button>
      </div>

      {expanded && (
        <div className="border-t border-zinc-200 px-3 py-3 dark:border-zinc-800">
          {task.items.length === 0 ? (
            <p className="text-sm text-zinc-500">No time entries yet.</p>
          ) : (
            <ul className="flex flex-col divide-y divide-zinc-100 dark:divide-zinc-800">
              {task.items.map((item) => (
                <li key={item.id} className="flex items-center gap-3 py-2 text-sm">
                  <span className="w-28 text-zinc-500">{formatDateTime(item.start_at)}</span>
                  <span className="flex-1 text-zinc-600 dark:text-zinc-400">
                    {item.manual_hours !== null
                      ? "Manual entry"
                      : item.end_at === null
                        ? `${formatTime(item.start_at)} → running`
                        : `${formatTime(item.start_at)} → ${formatTime(item.end_at)}`}
                  </span>
                  <span className={`font-mono ${isRunning(item) ? "text-blue-600" : ""}`}>
                    {formatDuration(itemSeconds(item, now))}
                  </span>
                  <button
                    onClick={() => void run(() => api.deleteItem(item.id))}
                    className="text-zinc-400 hover:text-red-600"
                    aria-label="Delete entry"
                  >
                    ✕
                  </button>
                </li>
              ))}
            </ul>
          )}

          <form onSubmit={addManualEntry} className="mt-3 flex flex-wrap items-center gap-2 text-sm">
            <span className="text-zinc-500">Add manually:</span>
            <input
              type="datetime-local"
              value={manualStart}
              onChange={(event) => setManualStart(event.target.value)}
              className="rounded-md border border-zinc-200 bg-transparent px-2 py-1 dark:border-zinc-700"
            />
            <input
              type="number"
              min="0.25"
              max="24"
              step="0.25"
              value={manualHours}
              onChange={(event) => setManualHours(event.target.value)}
              placeholder="Hours"
              className="w-24 rounded-md border border-zinc-200 bg-transparent px-2 py-1 dark:border-zinc-700"
            />
            <button
              type="submit"
              disabled={!(Number(manualHours) > 0)}
              className="rounded-md border border-zinc-300 px-3 py-1 hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
            >
              Add
            </button>
          </form>
        </div>
      )}
    </li>
  );
}
