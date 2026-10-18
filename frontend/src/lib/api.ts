export type TaskStatus = "todo" | "in_progress" | "done";

export type TaskItem = {
  id: number;
  task_id: number;
  start_at: string;
  end_at: string | null;
  manual_hours: number | null;
  created_at: string;
  updated_at: string;
};

// The list and detail endpoints add the two stats below; POST and PATCH /projects
// answer with the bare row, so those responses are typed as ProjectSummary.
export type ProjectSummary = {
  id: number;
  title: string;
  client_name: string;
  created_at: string;
  updated_at: string;
};

export type Project = ProjectSummary & {
  task_count: number;
  total_hours: number;
};

export type Task = {
  id: number;
  title: string;
  status: TaskStatus;
  created_at: string;
  updated_at: string;
  project_id: number | null;
  // Omitted by the API when the task has no project (the Go field is `omitempty`).
  project?: ProjectSummary | null;
  items: TaskItem[];
  total_hours: number;
  timer_count: number;
  running: boolean;
};

export type RunningTimer = {
  item_id: number;
  task_id: number;
  task_title: string;
  project_id: number | null;
  project_title: string | null;
  start_at: string;
};

export type Dashboard = {
  running_timers: RunningTimer[];
  totals: {
    today_hours: number;
    week_hours: number;
    all_time_hours: number;
  };
};

const API_URL = process.env.NEXT_PUBLIC_API_URL;

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  if (!API_URL) {
    throw new Error("NEXT_PUBLIC_API_URL is not set");
  }
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }
  return response.status === 204 ? (undefined as T) : response.json();
}

// The Go API answers errors as {"error": "..."}; anything else falls back to the status code.
async function errorMessage(response: Response): Promise<string> {
  try {
    const body: unknown = await response.json();
    if (typeof body === "object" && body !== null && "error" in body && typeof body.error === "string") {
      return body.error;
    }
  } catch {
    // not a JSON body
  }
  return `Request failed with ${response.status}`;
}

export const api = {
  getTasks: () => request<Task[]>("/tasks"),
  createTask: (title: string, projectId: number | null = null) =>
    request<Task>("/tasks", { method: "POST", body: JSON.stringify({ title, project_id: projectId }) }),
  updateTaskStatus: (id: number, status: TaskStatus) =>
    request<Task>(`/tasks/${id}`, { method: "PATCH", body: JSON.stringify({ status }) }),
  // projectId null takes the task out of its project: PUT is fine to repeat, unlike a POST.
  setTaskProject: (id: number, projectId: number | null) =>
    request<Task>(`/tasks/${id}/project`, {
      method: "PUT",
      body: JSON.stringify({ project_id: projectId }),
    }),
  deleteTask: (id: number) => request<void>(`/tasks/${id}`, { method: "DELETE" }),
  startTimer: (taskId: number) =>
    request<TaskItem>(`/tasks/${taskId}/start`, { method: "POST" }),
  stopTimer: (itemId: number) =>
    request<TaskItem>(`/items/${itemId}/stop`, { method: "POST" }),
  // Refreshes last_ping_at on every running timer; the server times them out when pings stop.
  pingTimers: () => request<{ running: number }>("/timers/ping", { method: "POST" }),
  addManualEntry: (taskId: number, startAt: string, hours: number) =>
    request<TaskItem>(`/tasks/${taskId}/items`, {
      method: "POST",
      body: JSON.stringify({ start_at: startAt, manual_hours: hours }),
    }),
  deleteItem: (itemId: number) => request<void>(`/items/${itemId}`, { method: "DELETE" }),

  getProjects: () => request<Project[]>("/projects"),
  createProject: (title: string, clientName: string) =>
    request<ProjectSummary>("/projects", {
      method: "POST",
      body: JSON.stringify({ title, client_name: clientName }),
    }),
  updateProject: (id: number, fields: { title?: string; client_name?: string }) =>
    request<ProjectSummary>(`/projects/${id}`, { method: "PATCH", body: JSON.stringify(fields) }),
  deleteProject: (id: number) => request<void>(`/projects/${id}`, { method: "DELETE" }),

  getDashboard: () => request<Dashboard>("/dashboard"),
};
