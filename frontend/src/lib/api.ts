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

export type Task = {
  id: number;
  title: string;
  status: TaskStatus;
  created_at: string;
  updated_at: string;
  items: TaskItem[];
  total_hours: number;
  timer_count: number;
  running: boolean;
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
  createTask: (title: string) =>
    request<Task>("/tasks", { method: "POST", body: JSON.stringify({ title }) }),
  updateTaskStatus: (id: number, status: TaskStatus) =>
    request<Task>(`/tasks/${id}`, { method: "PATCH", body: JSON.stringify({ status }) }),
  deleteTask: (id: number) => request<void>(`/tasks/${id}`, { method: "DELETE" }),
  startTimer: (taskId: number) =>
    request<TaskItem>(`/tasks/${taskId}/start`, { method: "POST" }),
  stopTimer: (itemId: number) =>
    request<TaskItem>(`/items/${itemId}/stop`, { method: "POST" }),
  addManualEntry: (taskId: number, startAt: string, hours: number) =>
    request<TaskItem>(`/tasks/${taskId}/items`, {
      method: "POST",
      body: JSON.stringify({ start_at: startAt, manual_hours: hours }),
    }),
  deleteItem: (itemId: number) => request<void>(`/items/${itemId}`, { method: "DELETE" }),
};
