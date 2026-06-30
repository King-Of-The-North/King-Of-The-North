/**
 * Mobile theme — sourced from @kotn/design-system tokens (single source of truth
 * shared with the web app). Legacy keys (backgroundElement/backgroundSelected/
 * textSecondary) are preserved so existing components keep working; new semantic
 * keys (accent, surface, border, …) power the ui/ primitives.
 */

import '@/global.css';

import { Platform } from 'react-native';
import {
  semantic,
  raw,
  family,
  spacing as tokenSpacing,
  radius as tokenRadius,
  scale,
  leading,
  weight,
  easing,
  duration,
} from '@kotn/design-system';

function modeColors(mode: 'light' | 'dark') {
  const c = semantic[mode];
  return {
    // Legacy keys (do not remove — consumed across existing components)
    text: c.text.primary,
    background: c.background,
    backgroundElement: c.surfaceElevated,
    backgroundSelected: mode === 'light' ? raw.neutral[100] : raw.neutral[700],
    textSecondary: c.text.secondary,
    // Semantic keys for ui/ primitives
    textTertiary: c.text.tertiary,
    textOnAccent: c.text.onAccent,
    textInverse: c.text.inverse,
    surface: c.surface,
    surfaceElevated: c.surfaceElevated,
    border: c.border,
    accent: c.accent,
    success: c.success,
    warning: c.warning,
    danger: c.danger,
  };
}

export const Colors = {
  light: modeColors('light'),
  dark: modeColors('dark'),
} as const;

export type ThemeColor = keyof typeof Colors.light & keyof typeof Colors.dark;

/**
 * Font families. `display` is the weight-mapped Neue Haas set (loaded via
 * expo-font in app/_layout). `sans` defaults to the regular face. Web reads the
 * CSS var the app exposes; native uses the registered family names.
 */
const monoFallback =
  Platform.select({ ios: 'ui-monospace', android: 'monospace', default: 'monospace' }) ??
  'monospace';

export const Fonts = {
  display: family.native,
  sans: family.native.regular,
  medium: family.native.medium,
  bold: family.native.bold,
  black: family.native.black,
  mono: monoFallback,
} as const;

/** Map for expo-font useFonts(): family name -> required asset. */
export const FontAssets = {
  [family.native.light]: require('../../assets/fonts/NeueHaasDisplay-Light.ttf'),
  [family.native.regular]: require('../../assets/fonts/NeueHaasDisplay-Roman.ttf'),
  [family.native.medium]: require('../../assets/fonts/NeueHaasDisplay-Medium.ttf'),
  [family.native.bold]: require('../../assets/fonts/NeueHaasDisplay-Bold.ttf'),
  [family.native.black]: require('../../assets/fonts/NeueHaasDisplay-Black.ttf'),
} as const;

// Re-export raw token groups so primitives can read them without re-importing the package.
export const TypeScale = scale;
export const Leading = leading;
export const Weight = weight;
export const Easing = easing;
export const Duration = duration;
export const Radius = tokenRadius;

/** Spacing — legacy 4px-grid keys kept for existing screens; token scale aliased. */
export const Spacing = {
  half: 2,
  one: tokenSpacing.xs, // 4
  two: tokenSpacing.sm, // 8
  three: tokenSpacing.md, // 16
  four: tokenSpacing.lg, // 24
  five: tokenSpacing.xl, // 32
  six: tokenSpacing['3xl'], // 64
} as const;

export const BottomTabInset = Platform.select({ ios: 50, android: 80 }) ?? 0;
export const MaxContentWidth = 800;
