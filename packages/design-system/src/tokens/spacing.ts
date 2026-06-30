/**
 * Spacing + radius tokens. Numbers are px (RN-native); web reads them as rem-ish
 * via the CSS generator. A 4px base grid with intentional jumps so layouts can
 * alternate dense and breathing sections (top-design pillar 2).
 */

export const spacing = {
  none: 0,
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
  '2xl': 48,
  '3xl': 64,
  '4xl': 96,
  '5xl': 128,
} as const;

export const radius = {
  none: 0,
  sm: 4,
  md: 8,
  lg: 16,
  xl: 24,
  full: 9999,
} as const;

/** Layout constants shared across apps. */
export const layout = {
  maxContentWidth: 1200,
  readingMeasure: 680,
} as const;

export type SpacingStep = keyof typeof spacing;
export type RadiusStep = keyof typeof radius;
