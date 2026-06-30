/**
 * Color tokens — warm-neutral base + single bold accent.
 *
 * Per top-design pillar 4: never pure #000/#fff. Warm black "ink" and cream "paper"
 * feel physical. One signature accent, reserved for CTAs and single-detail moments.
 * The semantic layer is built from opacity-based variants so secondary/tertiary text
 * stays consistent across surfaces, and is defined for BOTH light and dark.
 *
 * Accent is a single constant — swap `raw.accent` to re-brand the whole system.
 */

/** Raw palette — the only place literal hex values live. */
export const raw = {
  ink: '#0a0a0a', // warm black
  paper: '#faf9f6', // warm cream white
  accent: '#ff4d00', // signature electric orange — CTA / single-detail only
  accentInk: '#0a0a0a', // text/icon color that sits on the accent
  // Neutral ramp (warm-leaning grays), light → dark
  neutral: {
    50: '#f4f3f0',
    100: '#e9e7e2',
    200: '#d6d3cc',
    300: '#b8b4aa',
    400: '#8f8b80',
    500: '#6b675e',
    600: '#4d4a43',
    700: '#34322d',
    800: '#1f1e1b',
    900: '#141312',
  },
  // State colors (kept muted to live inside a neutral system)
  success: '#2f9e44',
  warning: '#e8a317',
  danger: '#e03131',
} as const;

/** rgba helper for opacity-based semantic variants. */
function rgba(hex: string, alpha: number): string {
  const h = hex.replace('#', '');
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

/** Semantic tokens per mode. Consumers should use these, not `raw`. */
export const semantic = {
  light: {
    background: raw.paper,
    surface: '#ffffff',
    surfaceElevated: raw.neutral[50],
    text: {
      primary: raw.ink,
      secondary: rgba(raw.ink, 0.6),
      tertiary: rgba(raw.ink, 0.4),
      onAccent: raw.accentInk,
      inverse: raw.paper,
    },
    border: rgba(raw.ink, 0.1),
    accent: raw.accent,
    success: raw.success,
    warning: raw.warning,
    danger: raw.danger,
  },
  dark: {
    background: raw.ink,
    surface: raw.neutral[900],
    surfaceElevated: raw.neutral[800],
    text: {
      primary: raw.paper,
      secondary: rgba(raw.paper, 0.6),
      tertiary: rgba(raw.paper, 0.4),
      onAccent: raw.accentInk,
      inverse: raw.ink,
    },
    border: rgba(raw.paper, 0.12),
    accent: raw.accent,
    success: raw.success,
    warning: raw.warning,
    danger: raw.danger,
  },
} as const;

export type ColorMode = keyof typeof semantic;
export type SemanticColors = (typeof semantic)['light'];
