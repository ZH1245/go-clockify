"use client";

import { useCallback, useEffect, useState } from "react";

import { api, type Project, type Task } from "@/lib/api";
import { formatDuration, taskSeconds } from "@/lib/time";

import { NewEntryForm } from "./new-entry-form";
import { TaskRow } from "./task-row";

export default function Tracker() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Starts at 0 because Next.js pre-renders this page on the server, where the current time must not be read.
  const [now, setNow] = useState(0);

  // Tasks and projects do not depend on each other, so both requests go out together.
  const loadData = useCallback(async () => {
    const [nextTasks, nextProjects] = await Promise.all([api.getTasks(), api.getProjects()]);
    setTasks(nextTasks);
    setProjects(nextProjects);
    setError(null);
  }, []);

  useEffect(() => {
    let ignore = false;
    async function initialLoad() {
      try {
        const [nextTasks, nextProjects] = await Promise.all([api.getTasks(), api.getProjects()]);
        if (!ignore) {
          setTasks(nextTasks);
          setProjects(nextProjects);
          setNow(Date.now());
        }
      } catch (err) {
        if (!ignore) setError(err instanceof Error ? err.message : String(err));
      } finally {
        if (!ignore) setLoading(false);
      }
    }
    // initialLoad handles its own errors, so the promise can't reject.
    void initialLoad();
    return () => {
      ignore = true;
    };
  }, []);

  const anyRunning = tasks.some((task) => task.running);
  useEffect(() => {
    if (!anyRunning) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [anyRunning]);

  // Every change goes to the API, then the list is reloaded so the UI always shows what the database has.
  const run = useCallback(
    async (action: () => Promise<unknown>) => {
      try {
        await action();
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
        return;
      }
      setNow(Date.now());
      try {
        await loadData();
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      }
    },
    [loadData],
  );

  const totalSeconds = tasks.reduce((sum, task) => sum + taskSeconds(task, now), 0);

  return (
    <main className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10">
      <header className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Time tracker</h1>
        <span className="font-mono text-sm text-zinc-500">Total {formatDuration(totalSeconds)}</span>
      </header>

      <NewEntryForm tasks={tasks} projects={projects} run={run} />

      {error && (
        <p className="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </p>
      )}

      {loading ? (
        <p className="text-sm text-zinc-500">Loading…</p>
      ) : tasks.length === 0 ? (
        <p className="text-sm text-zinc-500">No tasks yet. Type what you are working on above, then press Start or add hours manually.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {tasks.map((task) => (
            <TaskRow key={task.id} task={task} projects={projects} now={now} run={run} />
          ))}
        </ul>
      )}
    </main>
  );
}
