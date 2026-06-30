import type { CSSProperties } from "react";
import { formatMinorUnits } from "@kotn/design-system";

/**
 * Amount — renders integer minor units (kuruş, ADR-0003) as money with tabular
 * figures so digits align in tables/receipts. DISPLAY ONLY — uses the shared
 * integer formatter (no float math), never computes balances.
 */

export function Amount({
  minorUnits,
  currency = "₺",
  size = "heading",
  className = "",
  style,
}: {
  minorUnits: number | bigint;
  currency?: string;
  size?: "heading" | "title" | "display" | "body";
  className?: string;
  style?: CSSProperties;
}) {
  return (
    <span
      className={`text-text-primary inline-flex items-baseline gap-1 ${className}`}
      style={{
        fontVariantNumeric: "tabular-nums",
        fontSize: `var(--kotn-text-${size})`,
        letterSpacing: "var(--kotn-tracking-heading)",
        fontWeight: "var(--kotn-weight-medium)" as unknown as number,
        ...style,
      }}
    >
      <span className="text-text-tertiary" style={{ fontSize: "0.6em" }}>
        {currency}
      </span>
      {formatMinorUnits(minorUnits)}
    </span>
  );
}
