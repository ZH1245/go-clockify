"use client";

import { useCallback, useEffect, useState } from "react";

import { api, type Project, type Task, type TaskStatus } from "@/lib/api";
import { formatHours } from "@/lib/time";

const statusLabels: Record<TaskStatus, string> = {
  todo: "To do",
  in_progress: "In progress",
  done: "Done",
};

export default function Projects() {
  const [projects, setProjects] = useState<Project[]>([]);
  // There is no per-project tasks endpoint; GET /tasks carries project_id, so tasks are grouped here.
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [title, setTitle] = useState("");
  const [clientName, setClientName] = useState("");

  const [editingId, setEditingId] = useState<number | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editClient, setEditClient] = useState("");

  const [expandedId, setExpandedId] = useState<number | null>(null);

  const load = useCallback(async () => {
    try {
      const [nextProjects, nextTasks] = await Promise.all([api.getProjects(), api.getTasks()]);
      setProjects(nextProjects);
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
        const [nextProjects, nextTasks] = await Promise.all([api.getProjects(), api.getTasks()]);
        if (!ignore) {
          setProjects(nextProjects);
          setTasks(nextTasks);
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

  async function run(action: () => Promise<unknown>) {
    try {
      await action();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function create(event: React.FormEvent) {
    event.preventDefault();
    if (!title.trim()) return;
    await run(() => api.createProject(title.trim(), clientName.trim()));
    setTitle("");
    setClientName("");
  }

  function startEdit(project: Project) {
    setEditingId(project.id);
    setEditTitle(project.title);
    setEditClient(project.client_name);
  }

  async function saveEdit(id: number) {
    await run(() => api.updateProject(id, { title: editTitle.trim(), client_name: editClient.trim() }));
    setEditingId(null);
  }

  return (
    <main className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10">
      <h1 className="text-2xl font-semibold tracking-tight">Projects</h1>

      <form
        onSubmit={create}
        className="flex flex-wrap items-center gap-2 rounded-xl border border-zinc-200 bg-white p-3 text-sm dark:border-zinc-800 dark:bg-zinc-900"
      >
        <input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="Project name"
          className="min-w-40 flex-1 rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:border-blue-500 dark:border-zinc-700"
        />
        <input
          value={clientName}
          onChange={(event) => setClientName(event.target.value)}
          placeholder="Client (optional)"
          className="min-w-40 flex-1 rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:border-blue-500 dark:border-zinc-700"
        />
        <button
          type="submit"
          disabled={!title.trim()}
          className="rounded-lg bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          Add project
        </button>
      </form>

      {error && (
        <p className="rounded-lg border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {error}
        </p>
      )}

      {loading ? (
        <p className="text-sm text-zinc-500">Loading…</p>
      ) : projects.length === 0 ? (
        <p className="text-sm text-zinc-500">No projects yet. Create one above, then assign tasks to it.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {projects.map((project) => {
            const projectTasks = tasks.filter((task) => task.project_id === project.id);
            const expanded = expandedId === project.id;
            return (
              <li
                key={project.id}
                className="rounded-xl border border-zinc-200 bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900"
              >
                {editingId === project.id ? (
                  <div className="flex flex-wrap items-center gap-2 text-sm">
                    <input
                      value={editTitle}
                      onChange={(event) => setEditTitle(event.target.value)}
                      aria-label="Project name"
                      className="min-w-40 flex-1 rounded-md border border-zinc-300 bg-transparent px-2 py-1 dark:border-zinc-700"
                    />
                    <input
                      value={editClient}
                      onChange={(event) => setEditClient(event.target.value)}
                      aria-label="Client"
                      className="min-w-40 flex-1 rounded-md border border-zinc-300 bg-transparent px-2 py-1 dark:border-zinc-700"
                    />
                    <button
                      onClick={() => void saveEdit(project.id)}
                      disabled={!editTitle.trim()}
                      className="rounded-md bg-blue-600 px-3 py-1 text-white hover:bg-blue-700 disabled:opacity-50"
                    >
                      Save
                    </button>
                    <button
                      onClick={() => setEditingId(null)}
                      className="rounded-md border border-zinc-300 px-3 py-1 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => setExpandedId(expanded ? null : project.id)}
                      aria-expanded={expanded}
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="truncate font-medium">{project.title}</p>
                        {project.client_name && <p className="truncate text-xs text-zinc-500">{project.client_name}</p>}
                      </div>
                      <span className="text-zinc-400">{expanded ? "▾" : "▸"}</span>
                    </button>
                    <span className="shrink-0 text-xs text-zinc-500" title={`${project.task_count} tasks`}>
                      {project.task_count} {project.task_count === 1 ? "task" : "tasks"}
                    </span>
                    <span className="w-20 shrink-0 text-right font-mono text-sm">{formatHours(project.total_hours)}</span>
                    <button
                      onClick={() => startEdit(project)}
                      className="shrink-0 text-zinc-400 hover:text-blue-600"
                      aria-label={`Edit ${project.title}`}
                    >
                      ✎
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`Delete project "${project.title}"? Its tasks will be kept without a project.`)) {
                          void run(() => api.deleteProject(project.id));
                        }
                      }}
                      className="shrink-0 text-zinc-400 hover:text-red-600"
                      aria-label={`Delete ${project.title}`}
                    >
                      ✕
                    </button>
                  </div>
                )}

                {expanded && editingId !== project.id && (
                  <div className="mt-3 border-t border-zinc-200 pt-3 dark:border-zinc-800">
                    {projectTasks.length === 0 ? (
                      <p className="text-sm text-zinc-500">No tasks in this project. Assign them from the tracker.</p>
                    ) : (
                      <ul className="flex flex-col divide-y divide-zinc-100 dark:divide-zinc-800">
                        {projectTasks.map((task) => (
                          <li key={task.id} className="flex items-center gap-3 py-2 text-sm">
                            <span className="min-w-0 flex-1 truncate">{task.title}</span>
                            {task.running && (
                              <span className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-blue-500" title="Timer running" />
                            )}
                            <span className="shrink-0 text-xs text-zinc-500">{statusLabels[task.status]}</span>
                            <span className="w-20 shrink-0 text-right font-mono">{formatHours(task.total_hours)}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </main>
  );
}
