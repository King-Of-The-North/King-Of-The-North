import { Text as RNText, type TextProps, type TextStyle } from 'react-native';
import { family, scale, leading, tracking } from '@kotn/design-system';

import { useTheme } from '@/hooks/use-theme';

/**
 * Type primitives — RN siblings of the web Text/Heading (same names + props).
 * Custom fonts ship as one file per weight, so weight is expressed via fontFamily
 * (the Neue Haas face), not fontWeight. Tracking em → px (RN has no em units).
 */

type HeadingLevel = 'hero' | 'display' | 'title' | 'heading';
type TextVariant = 'bodyLg' | 'body' | 'small' | 'caption';
type Tone = 'primary' | 'secondary' | 'tertiary' | 'accent';

function emToPx(em: string, fontSize: number): number {
  return parseFloat(em) * fontSize;
}

const headingFace: Record<HeadingLevel, string> = {
  hero: family.native.black,
  display: family.native.bold,
  title: family.native.bold,
  heading: family.native.medium,
};

const headingScale: Record<HeadingLevel, { size: number; lead: number; track: string }> = {
  hero: { size: scale.hero.px, lead: leading.hero, track: tracking.hero },
  display: { size: scale.display.px, lead: leading.display, track: tracking.display },
  title: { size: scale.title.px, lead: leading.title, track: tracking.title },
  heading: { size: scale.heading.px, lead: leading.heading, track: tracking.heading },
};

function useTone(tone: Tone): string {
  const theme = useTheme();
  switch (tone) {
    case 'secondary':
      return theme.textSecondary;
    case 'tertiary':
      return theme.textTertiary;
    case 'accent':
      return theme.accent;
    default:
      return theme.text;
  }
}

export function Heading({
  level = 'title',
  tone = 'primary',
  style,
  ...rest
}: TextProps & { level?: HeadingLevel; tone?: Tone }) {
  const color = useTone(tone);
  const s = headingScale[level];
  const headingStyle: TextStyle = {
    color,
    fontFamily: headingFace[level],
    fontSize: s.size,
    lineHeight: s.size * s.lead,
    letterSpacing: emToPx(s.track, s.size),
  };
  return <RNText style={[headingStyle, style]} {...rest} />;
}

const textScale: Record<TextVariant, { size: number; face: string; caps?: boolean }> = {
  bodyLg: { size: scale.bodyLg.px, face: family.native.regular },
  body: { size: scale.body.px, face: family.native.regular },
  small: { size: scale.small.px, face: family.native.regular },
  caption: { size: scale.caption.px, face: family.native.medium, caps: true },
};

export function Text({
  variant = 'body',
  tone = 'primary',
  style,
  ...rest
}: TextProps & { variant?: TextVariant; tone?: Tone }) {
  const color = useTone(tone);
  const v = textScale[variant];
  const textStyle: TextStyle = {
    color,
    fontFamily: v.face,
    fontSize: v.size,
    lineHeight: v.size * leading.body,
    ...(v.caps
      ? { textTransform: 'uppercase', letterSpacing: emToPx(tracking.caps, v.size) }
      : null),
  };
  return <RNText style={[textStyle, style]} {...rest} />;
}
