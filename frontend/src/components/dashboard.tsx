"use client";

import { useCallback, useEffect, useState } from "react";

import { api, type Dashboard as DashboardData, type Task } from "@/lib/api";
import { elapsedSeconds, formatDuration, formatHours, formatTime, taskSeconds } from "@/lib/time";

export default function Dashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  // Task status is not part of GET /dashboard; the in-progress list comes from GET /tasks.
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // 0 until the first load, so server pre-rendering never reads the clock.
  const [now, setNow] = useState(0);

  const load = useCallback(async () => {
    try {
      const [nextData, nextTasks] = await Promise.all([api.getDashboard(), api.getTasks()]);
      setData(nextData);
      setTasks(nextTasks);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, []);

  useEffect(() => {
    let ignore = false;
    async function initialLoad() {
      try {
        const [nextData, nextTasks] = await Promise.all([api.getDashboard(), api.getTasks()]);
        if (!ignore) {
          setData(nextData);
          setTasks(nextTasks);
          setNow(Date.now());
        }
      } catch (err) {
        if (!ignore) setError(err instanceof Error ? err.message : String(err));
      } finally {
        if (!ignore) setLoading(false);
      }
    }
    void initialLoad();
    return () => {
      ignore = true;
    };
  }, []);

  // Tick once a second only while something is running, so idle dashboards stay quiet.
  const anyRunning = (data?.running_timers.length ?? 0) > 0;
  useEffect(() => {
    if (!anyRunning) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [anyRunning]);

  const inProgress = tasks.filter((task) => task.status === "in_progress");

  async function refresh() {
    await load();
    setNow(Date.now());
  }

  return (
    <main className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10">
      <header className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <button
          onClick={() => void refresh()}
          className="rounded-md border border-zinc-300 px-3 py-1 text-sm hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
        >
          Refresh
        </button>
      </header>

      {error && (
        <p className="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </p>
      )}

      {loading ? (
        <p className="text-sm text-zinc-500">Loading…</p>
      ) : (
        <>
          <section className="grid grid-cols-3 gap-3">
            <Stat label="Today" value={formatHours(data?.totals.today_hours ?? 0)} />
            <Stat label="This week" value={formatHours(data?.totals.week_hours ?? 0)} />
            <Stat label="All time" value={formatHours(data?.totals.all_time_hours ?? 0)} />
          </section>

          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-medium text-zinc-500">Running timers</h2>
            {anyRunning ? (
              <ul className="flex flex-col divide-y divide-zinc-100 rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
                {data!.running_timers.map((timer) => (
                  <li key={timer.item_id} className="flex items-center gap-3 px-4 py-3 text-sm">
                    <span className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-blue-500" />
                    <span className="min-w-0 flex-1 truncate font-medium">{timer.task_title}</span>
                    {timer.project_title && (
                      <span className="shrink-0 rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                        {timer.project_title}
                      </span>
                    )}
                    <span className="shrink-0 text-zinc-500">since {formatTime(timer.start_at)}</span>
                    <span className="w-20 shrink-0 text-right font-mono text-blue-600">
                      {formatDuration(elapsedSeconds(timer.start_at, now))}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-zinc-500">Nothing is running right now.</p>
            )}
          </section>

          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-medium text-zinc-500">In progress</h2>
            {inProgress.length > 0 ? (
              <ul className="flex flex-col divide-y divide-zinc-100 rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900">
                {inProgress.map((task) => (
                  <li key={task.id} className="flex items-center gap-3 px-4 py-3 text-sm">
                    {task.running && <span className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-blue-500" />}
                    <span className="min-w-0 flex-1 truncate font-medium">{task.title}</span>
                    {task.project && (
                      <span className="shrink-0 rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                        {task.project.title}
                      </span>
                    )}
                    <span className={`w-20 shrink-0 text-right font-mono ${task.running ? "text-blue-600" : ""}`}>
                      {formatDuration(taskSeconds(task, now))}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-zinc-500">No tasks are marked in progress.</p>
            )}
          </section>
        </>
      )}
    </main>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-zinc-200 bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900">
      <p className="text-xs text-zinc-500">{label}</p>
      <p className="mt-1 font-mono text-lg">{value}</p>
    </div>
  );
}
