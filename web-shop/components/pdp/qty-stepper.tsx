"use client";
import { Minus, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

type Props = {
  value: number;
  onChange: (n: number) => void;
  max: number;
  min?: number;
  disabled?: boolean;
};

export function QtyStepper({ value, onChange, max, min = 1, disabled }: Props) {
  const atMin = disabled || value <= min;
  const atMax = disabled || value >= max;
  return (
    <div className="inline-flex items-center rounded-xl border border-border overflow-hidden">
      <button
        type="button"
        onClick={() => onChange(Math.max(min, value - 1))}
        disabled={atMin}
        aria-label="Decrease quantity"
        className={cn(
          "size-10 grid place-items-center",
          atMin ? "text-muted cursor-not-allowed" : "hover:bg-brand-50",
        )}
      >
        <Minus className="size-4" aria-hidden />
      </button>
      <span aria-live="polite" className="w-10 text-center text-sm font-medium tabular-nums">
        {value}
      </span>
      <button
        type="button"
        onClick={() => onChange(Math.min(max, value + 1))}
        disabled={atMax}
        aria-label="Increase quantity"
        className={cn(
          "size-10 grid place-items-center",
          atMax ? "text-muted cursor-not-allowed" : "hover:bg-brand-50",
        )}
      >
        <Plus className="size-4" aria-hidden />
      </button>
    </div>
  );
}
