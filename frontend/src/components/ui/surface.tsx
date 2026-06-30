import type { CSSProperties, ReactNode } from "react";

/**
 * Surface — a contained panel. `elevated` lifts it off the background; `bordered`
 * adds the 10%-opacity hairline border.
 */
export function Surface({
  elevated = false,
  bordered = true,
  className = "",
  style,
  children,
}: {
  elevated?: boolean;
  bordered?: boolean;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}) {
  return (
    <div
      className={`rounded-lg ${elevated ? "bg-surface-elevated" : "bg-surface"} ${
        bordered ? "border border-border" : ""
      } ${className}`}
      style={{ padding: "var(--kotn-space-lg)", ...style }}
    >
      {children}
    </div>
  );
}
