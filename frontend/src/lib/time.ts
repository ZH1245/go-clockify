import type { Task, TaskItem } from "./api";

export function isRunning(item: TaskItem): boolean {
  return item.end_at === null && item.manual_hours === null;
}

// The API's total_hours leaves out running timers, so the UI adds their live time on every tick.
export function taskSeconds(task: Task, now: number): number {
  const running = task.items.filter(isRunning);
  return task.total_hours * 3600 + running.reduce((sum, item) => sum + liveSeconds(item, now), 0);
}

export function itemSeconds(item: TaskItem, now: number): number {
  if (item.manual_hours !== null) return item.manual_hours * 3600;
  if (item.end_at === null) return liveSeconds(item, now);
  return (Date.parse(item.end_at) - Date.parse(item.start_at)) / 1000;
}

function liveSeconds(item: TaskItem, now: number): number {
  return Math.max(0, (now - Date.parse(item.start_at)) / 1000);
}

export function formatDuration(seconds: number): string {
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  return [h, m, s].map((part) => String(part).padStart(2, "0")).join(":");
}

export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

// Value for <input type="datetime-local">, in the browser's local time.
export function nowForDateTimeInput(): string {
  const now = new Date();
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
  return now.toISOString().slice(0, 16);
}
