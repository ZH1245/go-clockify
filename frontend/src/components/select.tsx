"use client";

import * as RadixSelect from "@radix-ui/react-select";

export type SelectOption<T extends string> = { value: T; label: string };

type Props<T extends string> = {
  value: T;
  options: SelectOption<T>[];
  onValueChange: (value: T) => void;
  label: string;
  className?: string;
};

// Radix renders its own popup, so unlike a native <select> the open list follows the
// light/dark colors below instead of the browser's default white menu.
export function Select<T extends string>({ value, options, onValueChange, label, className = "px-2 py-1 text-xs" }: Props<T>) {
  return (
    // Radix only ever reports values from `options`, so narrowing back to T is safe.
    <RadixSelect.Root value={value} onValueChange={(next) => onValueChange(next as T)}>
      <RadixSelect.Trigger
        aria-label={label}
        className={`inline-flex items-center justify-between gap-2 rounded-md border border-zinc-300 bg-white text-zinc-900 outline-none focus:border-blue-500 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 ${className}`}
      >
        <RadixSelect.Value />
        <RadixSelect.Icon className="text-zinc-400">▾</RadixSelect.Icon>
      </RadixSelect.Trigger>
      <RadixSelect.Portal>
        <RadixSelect.Content
          position="popper"
          sideOffset={4}
          className="z-50 max-h-(--radix-select-content-available-height) min-w-(--radix-select-trigger-width) overflow-hidden rounded-md border border-zinc-200 bg-white text-sm text-zinc-900 shadow-lg dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
        >
          <RadixSelect.Viewport className="p-1">
            {options.map((option) => (
              <RadixSelect.Item
                key={option.value}
                value={option.value}
                className="relative flex cursor-default select-none items-center rounded py-1.5 pr-2 pl-6 outline-none data-highlighted:bg-zinc-100 dark:data-highlighted:bg-zinc-800"
              >
                <RadixSelect.ItemIndicator className="absolute left-1.5 text-blue-600">✓</RadixSelect.ItemIndicator>
                <RadixSelect.ItemText>{option.label}</RadixSelect.ItemText>
              </RadixSelect.Item>
            ))}
          </RadixSelect.Viewport>
        </RadixSelect.Content>
      </RadixSelect.Portal>
    </RadixSelect.Root>
  );
}
