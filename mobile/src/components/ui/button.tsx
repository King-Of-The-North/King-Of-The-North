import { Pressable, type PressableProps, type ViewStyle } from 'react-native';
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from 'react-native-reanimated';
import { family, scale, radius, easing, duration } from '@kotn/design-system';

import { useTheme } from '@/hooks/use-theme';

/**
 * Button — RN sibling of the web Button. Press scale uses the shared quart-out
 * easing (no default curve). Accent variant = the single hero CTA.
 */
type Variant = 'primary' | 'accent' | 'ghost';

const press = Easing.bezier(...(easing.quartOut.bezier as [number, number, number, number]));

export function Button({
  variant = 'primary',
  children,
  style,
  onPressIn,
  onPressOut,
  ...rest
}: PressableProps & { variant?: Variant; children: React.ReactNode }) {
  const theme = useTheme();
  const s = useSharedValue(1);

  // `.set()/.get()` (Reanimated 4) instead of `.value =` — keeps the React
  // Compiler lint happy (it flags direct assignment to the shared value).
  const animatedStyle = useAnimatedStyle(() => ({ transform: [{ scale: s.get() }] }));

  const colors: Record<Variant, { bg: string; fg: string; border?: string }> = {
    primary: { bg: theme.text, fg: theme.textInverse },
    accent: { bg: theme.accent, fg: theme.textOnAccent },
    ghost: { bg: 'transparent', fg: theme.text, border: theme.border },
  };
  const c = colors[variant];

  const containerStyle: ViewStyle = {
    backgroundColor: c.bg,
    borderRadius: radius.md,
    paddingVertical: 14,
    paddingHorizontal: 24,
    alignItems: 'center',
    justifyContent: 'center',
    ...(c.border ? { borderWidth: 1, borderColor: c.border } : null),
  };

  return (
    <Pressable
      onPressIn={(e) => {
        s.set(withTiming(0.97, { duration: duration.instant, easing: press }));
        onPressIn?.(e);
      }}
      onPressOut={(e) => {
        s.set(withTiming(1, { duration: duration.fast, easing: press }));
        onPressOut?.(e);
      }}
      {...rest}>
      <Animated.View style={[containerStyle, animatedStyle, style as ViewStyle]}>
        <Animated.Text
          style={{ color: c.fg, fontFamily: family.native.medium, fontSize: scale.body.px }}>
          {children}
        </Animated.Text>
      </Animated.View>
    </Pressable>
  );
}
