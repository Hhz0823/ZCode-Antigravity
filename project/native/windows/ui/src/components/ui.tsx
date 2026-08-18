import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../lib/utils";

const buttonVariants = cva(
  "inline-flex select-none items-center justify-center gap-2 whitespace-nowrap rounded-xl text-sm font-medium outline-none transition-[background-color,border-color,color,box-shadow,transform] duration-200 focus-visible:ring-2 focus-visible:ring-sky-300/60 disabled:pointer-events-none disabled:opacity-45 active:scale-[.985]",
  {
    variants: {
      variant: {
        primary: "border border-sky-300/30 bg-gradient-to-b from-sky-400/95 to-blue-600/95 text-white shadow-[0_14px_30px_-18px_rgba(56,189,248,.95)] hover:from-sky-300 hover:to-blue-500",
        secondary: "border border-white/12 bg-white/[.075] text-slate-100 shadow-inner shadow-white/[.025] hover:bg-white/[.12]",
        ghost: "border border-transparent text-slate-300 hover:bg-white/[.08] hover:text-white",
        danger: "border border-rose-300/20 bg-rose-400/10 text-rose-100 hover:bg-rose-400/18",
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
  return <section className={cn("glass-card rounded-[22px] border border-white/10", className)} {...props}>{children}</section>;
}

export function CardHeader({ eyebrow, title, description, action }: { eyebrow?: string; title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 px-5 pt-5">
      <div className="min-w-0">
        {eyebrow && <p className="mb-1 text-[10px] font-semibold uppercase tracking-[.2em] text-sky-300/80">{eyebrow}</p>}
        <h2 className="truncate text-lg font-semibold tracking-[-.02em] text-white">{title}</h2>
        {description && <p className="mt-1 text-xs leading-5 text-slate-400">{description}</p>}
      </div>
      {action}
    </div>
  );
}

export function Badge({ tone = "neutral", children }: { tone?: "good" | "warn" | "bad" | "neutral" | "blue"; children: ReactNode }) {
  const tones = {
    good: "border-emerald-300/20 bg-emerald-400/10 text-emerald-200",
    warn: "border-amber-300/20 bg-amber-400/10 text-amber-200",
    bad: "border-rose-300/20 bg-rose-400/10 text-rose-200",
    blue: "border-sky-300/20 bg-sky-400/10 text-sky-200",
    neutral: "border-white/10 bg-white/[.06] text-slate-300",
  };
  return <span className={cn("inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-medium", tones[tone])}>{children}</span>;
}

export function Progress({ value, warning = false }: { value: number; warning?: boolean }) {
  const normalized = Math.max(0, Math.min(100, value));
  return (
    <div className="h-2 overflow-hidden rounded-full border border-white/[.06] bg-slate-950/35">
      <div
        className={cn("h-full rounded-full transition-[width] duration-700 ease-out", warning ? "bg-gradient-to-r from-rose-500 to-amber-400" : "bg-gradient-to-r from-cyan-300 via-sky-400 to-blue-500")}
        style={{ width: `${normalized}%` }}
      />
    </div>
  );
}

export function Switch({ checked, onChange, label }: { checked: boolean; onChange: (checked: boolean) => void; label: string }) {
  return (
    <button type="button" role="switch" aria-checked={checked} onClick={() => onChange(!checked)} className="flex w-full items-center justify-between gap-4 rounded-xl border border-white/[.08] bg-white/[.045] px-3 py-2.5 text-left transition hover:bg-white/[.075]">
      <span className="text-sm text-slate-200">{label}</span>
      <span className={cn("relative h-6 w-11 rounded-full border transition", checked ? "border-sky-300/30 bg-sky-500/80" : "border-white/10 bg-slate-700/80")}>
        <span className={cn("absolute top-0.5 size-4.5 rounded-full bg-white shadow transition-transform", checked ? "translate-x-[21px]" : "translate-x-0.5")} />
      </span>
    </button>
  );
}
