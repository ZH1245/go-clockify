"use client";

import type { Project } from "@/lib/api";

import { Select } from "./select";

type Props = {
  projects: Project[];
  value: number | null;
  onChange: (projectId: number | null) => void;
  className?: string;
  label?: string;
};

// Radix reserves "" for clearing the selection, so "none" stands for "no project" and
// every other value is Number()ed back into a project id.
const NO_PROJECT = "none";

export function ProjectSelect({ projects, value, onChange, className, label = "Project" }: Props) {
  return (
    <Select
      value={value === null ? NO_PROJECT : String(value)}
      options={[
        { value: NO_PROJECT, label: "No project" },
        ...projects.map((project) => ({ value: String(project.id), label: project.title })),
      ]}
      onValueChange={(next) => onChange(next === NO_PROJECT ? null : Number(next))}
      label={label}
      className={className}
    />
  );
}
