import { View, type ViewProps, type ViewStyle } from 'react-native';
import { radius, spacing } from '@kotn/design-system';

import { useTheme } from '@/hooks/use-theme';

/** Surface — contained panel. RN sibling of the web Surface. */
export function Surface({
  elevated = false,
  bordered = true,
  style,
  ...rest
}: ViewProps & { elevated?: boolean; bordered?: boolean }) {
  const theme = useTheme();
  const surfaceStyle: ViewStyle = {
    backgroundColor: elevated ? theme.surfaceElevated : theme.surface,
    borderRadius: radius.lg,
    padding: spacing.lg,
    ...(bordered ? { borderWidth: 1, borderColor: theme.border } : null),
  };
  return <View style={[surfaceStyle, style]} {...rest} />;
}
