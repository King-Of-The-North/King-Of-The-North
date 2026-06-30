/**
 * Typography tokens — Neue Haas Grotesk Display as the single type voice.
 *
 * Per top-design pillar 1: typography IS the design. Dramatic scale contrast
 * (display:body ≥ 10:1), tight negative tracking on large type, generous body
 * leading. The fluid scale is CSS clamp() for web; the `px` map gives mobile a
 * fixed equivalent (RN has no clamp()).
 *
 * Family is referenced by token so swapping faces = one change. The `family.cssVar`
 * is what next/font/local exposes on web; `family.native` are the registered
 * expo-font family names on mobile.
 */

export const family = {
  /** CSS custom property set by next/font/local in the web app. */
  cssVar: 'var(--font-display)',
  /** Generic fallback stack (also the FOUT fallback before the face loads). */
  fallback:
    "'Helvetica Neue', Helvetica, 'Arial Nova', Arial, system-ui, sans-serif",
  /** expo-font family names registered on mobile, keyed by weight. */
  native: {
    light: 'NeueHaasDisplay-Light',
    regular: 'NeueHaasDisplay-Roman',
    medium: 'NeueHaasDisplay-Medium',
    bold: 'NeueHaasDisplay-Bold',
    black: 'NeueHaasDisplay-Black',
  },
} as const;

/** Numeric weights mapped to the shipped faces. */
export const weight = {
  light: 300,
  regular: 400,
  medium: 500,
  bold: 700,
  black: 900,
} as const;

/**
 * Fluid type scale. `fluid` = CSS clamp() (web). `min`/`max` px feed the clamp
 * and also serve as the mobile fixed sizes (`px` = the max, used on phones).
 * hero/display fill the viewport; body stays intimate → ≥10:1 contrast.
 */
export const scale = {
  caption: { fluid: 'clamp(0.75rem, 0.72rem + 0.15vw, 0.8125rem)', px: 13 },
  small: { fluid: 'clamp(0.8125rem, 0.78rem + 0.18vw, 0.875rem)', px: 14 },
  body: { fluid: 'clamp(1rem, 0.96rem + 0.2vw, 1.0625rem)', px: 17 },
  bodyLg: { fluid: 'clamp(1.125rem, 1.05rem + 0.35vw, 1.25rem)', px: 20 },
  heading: { fluid: 'clamp(1.5rem, 1.25rem + 1.2vw, 2rem)', px: 32 },
  title: { fluid: 'clamp(2rem, 1.5rem + 2.4vw, 3rem)', px: 48 },
  display: { fluid: 'clamp(3rem, 1.5rem + 6vw, 6rem)', px: 72 },
  hero: { fluid: 'clamp(4rem, 1rem + 12vw, 11rem)', px: 96 },
} as const;

/** Tracking (letter-spacing) — scales inversely with size, em units. */
export const tracking = {
  hero: '-0.04em',
  display: '-0.035em',
  title: '-0.03em',
  heading: '-0.02em',
  body: '0em',
  caps: '0.1em',
} as const;

/** Leading (line-height) — tight for impact, comfortable for reading. */
export const leading = {
  hero: 0.9,
  display: 0.95,
  title: 1.05,
  heading: 1.1,
  body: 1.6,
  relaxed: 1.8,
} as const;

export type ScaleStep = keyof typeof scale;
