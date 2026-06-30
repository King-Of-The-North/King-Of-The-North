"use client";

import type { ButtonHTMLAttributes, ReactNode } from "react";

/**
 * Button — three variants. Custom-easing transition (no default `ease`), real
 * focus-visible state (from globals.css), accent reserved for the primary CTA.
 */
type Variant = "primary" | "accent" | "ghost";

const base =
  "inline-flex items-center justify-center gap-2 rounded-md font-medium " +
  "px-6 py-3 cursor-pointer select-none " +
  "transition-[transform,background-color,color,opacity] " +
  "[transition-duration:var(--kotn-duration-fast)] " +
  "[transition-timing-function:var(--kotn-ease-quartOut)] " +
  "active:scale-[0.97] disabled:opacity-40 disabled:pointer-events-none";

const variants: Record<Variant, string> = {
  // Solid ink — the default action.
  primary:
    "bg-text-primary text-text-inverse hover:opacity-90",
  // Signature accent — the single hero CTA. Use sparingly.
  accent:
    "bg-accent text-text-on-accent hover:brightness-105",
  // Quiet — secondary actions.
  ghost:
    "bg-transparent text-text-primary border border-border hover:bg-surface-elevated",
};

export function Button({
  variant = "primary",
  className = "",
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  children: ReactNode;
}) {
  return (
    <button
      className={`${base} ${variants[variant]} ${className}`}
      style={{ fontSize: "var(--kotn-text-body)" }}
      {...props}
    >
      {children}
    </button>
  );
}
