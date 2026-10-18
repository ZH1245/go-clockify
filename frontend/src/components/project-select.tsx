"use client";

import type { Project } from "@/lib/api";

type Props = {
  projects: Project[];
  value: number | null;
  onChange: (projectId: number | null) => void;
  className?: string;
  label?: string;
};

// A <select> works with strings, so "" means "no project" and every other value is
// Number()ed back into a project id.
export function ProjectSelect({ projects, value, onChange, className, label = "Project" }: Props) {
  return (
    <select
      value={value ?? ""}
      aria-label={label}
      onChange={(event) => onChange(event.target.value === "" ? null : Number(event.target.value))}
      className={
        className ??
        "rounded-md border border-zinc-200 bg-transparent px-2 py-1 text-xs dark:border-zinc-700"
      }
    >
      <option value="">No project</option>
      {projects.map((project) => (
        <option key={project.id} value={project.id}>
          {project.title}
        </option>
      ))}
    </select>
  );
}
