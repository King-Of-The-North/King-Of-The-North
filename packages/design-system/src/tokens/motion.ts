/**
 * Motion tokens. top-design bans `ease`/`linear` — every transition uses a custom
 * curve. `cssBezier` strings feed web transitions; `bezier` 4-tuples feed
 * react-native-reanimated's `Easing.bezier(...)` on mobile.
 */

export const easing = {
  /** Expo out — default for reveals / enters. */
  expoOut: { bezier: [0.16, 1, 0.3, 1], cssBezier: 'cubic-bezier(0.16, 1, 0.3, 1)' },
  /** Quart out — snappier, good for hover/press. */
  quartOut: { bezier: [0.25, 1, 0.5, 1], cssBezier: 'cubic-bezier(0.25, 1, 0.5, 1)' },
  /** Expo in-out — symmetric, for toggles/morphs. */
  expoInOut: { bezier: [0.87, 0, 0.13, 1], cssBezier: 'cubic-bezier(0.87, 0, 0.13, 1)' },
} as const;

/** Durations in ms. */
export const duration = {
  instant: 80,
  fast: 180,
  base: 320,
  slow: 600,
  reveal: 900,
} as const;

export type EasingName = keyof typeof easing;
export type DurationName = keyof typeof duration;
