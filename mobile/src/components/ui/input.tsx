import { useState } from 'react';
import { TextInput, View, type TextInputProps, type ViewStyle } from 'react-native';
import { family, radius, scale, spacing } from '@kotn/design-system';

import { useTheme } from '@/hooks/use-theme';
import { Text } from './text';

/** Input — branded TextInput. RN sibling of the web Input (label + hint + accent focus). */
export function Input({
  label,
  hint,
  style,
  onFocus,
  onBlur,
  ...rest
}: TextInputProps & { label?: string; hint?: string }) {
  const theme = useTheme();
  const [focused, setFocused] = useState(false);

  const inputStyle: ViewStyle = {
    backgroundColor: theme.surface,
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: focused ? theme.accent : theme.border,
    paddingVertical: 12,
    paddingHorizontal: 16,
  };

  return (
    <View style={{ gap: spacing.sm }}>
      {label ? (
        <Text variant="caption" tone="secondary">
          {label}
        </Text>
      ) : null}
      <TextInput
        placeholderTextColor={theme.textTertiary}
        style={[
          inputStyle,
          { color: theme.text, fontFamily: family.native.regular, fontSize: scale.body.px },
          style,
        ]}
        onFocus={(e) => {
          setFocused(true);
          onFocus?.(e);
        }}
        onBlur={(e) => {
          setFocused(false);
          onBlur?.(e);
        }}
        {...rest}
      />
      {hint ? (
        <Text variant="small" tone="tertiary">
          {hint}
        </Text>
      ) : null}
    </View>
  );
}
