import type { InputHTMLAttributes } from "react";

/**
 * Input — fully branded (no default form chrome). Label + optional hint, custom
 * focus border on the accent. top-design: design every input, never ship unstyled.
 */
export function Input({
  label,
  hint,
  className = "",
  id,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
  hint?: string;
}) {
  const inputId = id ?? props.name;
  return (
    <div className="flex flex-col gap-2">
      {label ? (
        <label
          htmlFor={inputId}
          className="text-text-secondary"
          style={{
            fontSize: "var(--kotn-text-caption)",
            letterSpacing: "var(--kotn-tracking-caps)",
            textTransform: "uppercase",
          }}
        >
          {label}
        </label>
      ) : null}
      <input
        id={inputId}
        className={
          "w-full rounded-md bg-surface text-text-primary border border-border " +
          "px-4 py-3 outline-none placeholder:text-text-tertiary " +
          "transition-[border-color] [transition-duration:var(--kotn-duration-fast)] " +
          "[transition-timing-function:var(--kotn-ease-quartOut)] " +
          "focus:border-accent " +
          className
        }
        style={{ fontSize: "var(--kotn-text-body)" }}
        {...props}
      />
      {hint ? (
        <span className="text-text-tertiary" style={{ fontSize: "var(--kotn-text-small)" }}>
          {hint}
        </span>
      ) : null}
    </div>
  );
}
