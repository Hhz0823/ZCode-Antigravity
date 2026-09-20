import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../lib/utils";

const buttonVariants = cva(
  "inline-flex select-none items-center justify-center gap-2 whitespace-nowrap rounded-xl text-sm font-medium outline-none transition-[background-color,border-color,color,box-shadow,transform] duration-200 focus-visible:ring-2 focus-visible:ring-sky-300/60 disabled:pointer-events-none disabled:opacity-45 active:scale-[.985]",
  {
    variants: {
      variant: {
        primary: "border border-blue-600 bg-blue-600 text-white shadow-sm hover:bg-blue-700",
        secondary: "border border-slate-300 bg-white text-slate-700 shadow-sm hover:bg-slate-100",
        ghost: "border border-transparent text-slate-700 hover:bg-slate-100 hover:text-slate-900",
        danger: "border border-rose-200 bg-rose-50 text-rose-800 hover:bg-rose-100",
      },
      size: {
        sm: "h-9 px-3",
        md: "h-11 px-4",
        lg: "h-12 px-5 text-[15px]",
        icon: "size-9",
      },
    },
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

export function Button({ className, variant, size, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof buttonVariants>) {
  return <button className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}

export function Card({ className, children, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <section className={cn("glass-card rounded-2xl border border-white/10", className)} {...props}>{children}</section>;
}

export function CardHeader({ title, description, action }: { eyebrow?: string; title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 px-5 pt-5">
      <div className="min-w-0">
        <h2 className="truncate text-base font-semibold tracking-[-.02em] text-white">{title}</h2>
        {description && <p className="mt-1 text-xs leading-5 text-slate-400">{description}</p>}
      </div>
      {action}
    </div>
  );
}

export function Badge({ tone = "neutral", children }: { tone?: "good" | "warn" | "bad" | "neutral" | "blue"; children: ReactNode }) {
  const tones = {
    good: "border-emerald-200 bg-emerald-50 text-emerald-800",
    warn: "border-amber-200 bg-amber-50 text-amber-800",
    bad: "border-rose-200 bg-rose-50 text-rose-800",
    blue: "border-sky-200 bg-sky-50 text-sky-800",
    neutral: "border-slate-200 bg-slate-50 text-slate-700",
  };
  return <span className={cn("inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-medium", tones[tone])}>{children}</span>;
}

export function Progress({ value, warning = false }: { value: number; warning?: boolean }) {
  const normalized = Math.max(0, Math.min(100, value));
  return (
    <div className="h-2 overflow-hidden rounded-full border border-white/[.06] bg-slate-100">
      <div
        className={cn("h-full rounded-full transition-[width] duration-700 ease-out", warning ? "bg-amber-500" : "bg-blue-500")}
        style={{ width: `${normalized}%` }}
      />
    </div>
  );
}

export function Switch({ checked, onChange, label }: { checked: boolean; onChange: (checked: boolean) => void; label: string }) {
  return (
    <button type="button" role="switch" aria-checked={checked} onClick={() => onChange(!checked)} className="flex w-full items-center justify-between gap-4 rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-left transition hover:bg-slate-50">
      <span className="text-sm text-slate-200">{label}</span>
      <span className={cn("relative h-6 w-11 rounded-full border transition", checked ? "border-blue-600 bg-blue-600" : "border-slate-300 bg-slate-300")}>
        <span className={cn("absolute top-0.5 size-4.5 rounded-full bg-white shadow transition-transform", checked ? "translate-x-[21px]" : "translate-x-0.5")} />
      </span>
    </button>
  );
}
