import type { CSSProperties, ElementType, ReactNode } from "react";

/**
 * Type primitives driven by design-system tokens (exposed as CSS vars in tokens.css).
 * Heading = display/architectural type (tight tracking, short leading, balance).
 * Text = reading/UI copy.
 */

type HeadingLevel = "hero" | "display" | "title" | "heading";

const headingStyle: Record<HeadingLevel, CSSProperties> = {
  hero: {
    fontSize: "var(--kotn-text-hero)",
    letterSpacing: "var(--kotn-tracking-hero)",
    lineHeight: "var(--kotn-leading-hero)",
    fontWeight: "var(--kotn-weight-black)" as unknown as number,
  },
  display: {
    fontSize: "var(--kotn-text-display)",
    letterSpacing: "var(--kotn-tracking-display)",
    lineHeight: "var(--kotn-leading-display)",
    fontWeight: "var(--kotn-weight-bold)" as unknown as number,
  },
  title: {
    fontSize: "var(--kotn-text-title)",
    letterSpacing: "var(--kotn-tracking-title)",
    lineHeight: "var(--kotn-leading-title)",
    fontWeight: "var(--kotn-weight-bold)" as unknown as number,
  },
  heading: {
    fontSize: "var(--kotn-text-heading)",
    letterSpacing: "var(--kotn-tracking-heading)",
    lineHeight: "var(--kotn-leading-heading)",
    fontWeight: "var(--kotn-weight-medium)" as unknown as number,
  },
};

const defaultTag: Record<HeadingLevel, ElementType> = {
  hero: "h1",
  display: "h1",
  title: "h2",
  heading: "h3",
};

export function Heading({
  level = "title",
  as,
  className = "",
  style,
  children,
}: {
  level?: HeadingLevel;
  as?: ElementType;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}) {
  const Tag = as ?? defaultTag[level];
  return (
    <Tag
      className={`text-text-primary text-balance ${className}`}
      style={{ ...headingStyle[level], ...style }}
    >
      {children}
    </Tag>
  );
}

type TextVariant = "bodyLg" | "body" | "small" | "caption";
type TextTone = "primary" | "secondary" | "tertiary" | "accent";

const textStyle: Record<TextVariant, CSSProperties> = {
  bodyLg: { fontSize: "var(--kotn-text-bodyLg)", lineHeight: "var(--kotn-leading-body)" },
  body: { fontSize: "var(--kotn-text-body)", lineHeight: "var(--kotn-leading-body)" },
  small: { fontSize: "var(--kotn-text-small)", lineHeight: "var(--kotn-leading-body)" },
  caption: {
    fontSize: "var(--kotn-text-caption)",
    lineHeight: "var(--kotn-leading-body)",
    letterSpacing: "var(--kotn-tracking-caps)",
    textTransform: "uppercase",
  },
};

const toneClass: Record<TextTone, string> = {
  primary: "text-text-primary",
  secondary: "text-text-secondary",
  tertiary: "text-text-tertiary",
  accent: "text-accent",
};

export function Text({
  variant = "body",
  tone = "primary",
  as: Tag = "p",
  className = "",
  style,
  children,
}: {
  variant?: TextVariant;
  tone?: TextTone;
  as?: ElementType;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}) {
  return (
    <Tag
      className={`${toneClass[tone]} ${className}`}
      style={{ ...textStyle[variant], ...style }}
    >
      {children}
    </Tag>
  );
}
