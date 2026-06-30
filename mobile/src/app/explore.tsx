import { Platform, ScrollView, StyleSheet, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Amount, Button, Heading, Input, Surface, Text } from '@/components/ui';
import { BottomTabInset, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/**
 * Styleguide — the mobile design-system proof harness (RN siblings of the web
 * /styleguide). Renders every token + primitive: type scale, color, buttons,
 * input, surfaces, and the kuruş Amount.
 */
export default function StyleguideScreen() {
  const safeAreaInsets = useSafeAreaInsets();
  const theme = useTheme();
  const insets = {
    ...safeAreaInsets,
    bottom: safeAreaInsets.bottom + BottomTabInset + Spacing.three,
  };

  const contentPlatformStyle = Platform.select({
    android: { paddingTop: insets.top, paddingBottom: insets.bottom },
    web: { paddingTop: Spacing.six, paddingBottom: Spacing.four },
    default: { paddingBottom: insets.bottom },
  });

  const swatches: { name: string; color: string }[] = [
    { name: 'background', color: theme.background },
    { name: 'surface', color: theme.surface },
    { name: 'elevated', color: theme.surfaceElevated },
    { name: 'text', color: theme.text },
    { name: 'accent', color: theme.accent },
    { name: 'success', color: theme.success },
    { name: 'warning', color: theme.warning },
    { name: 'danger', color: theme.danger },
  ];

  return (
    <ScrollView
      style={{ backgroundColor: theme.background }}
      contentInset={insets}
      contentContainerStyle={[styles.content, contentPlatformStyle]}>
      <Heading level="display">King of the North</Heading>
      <Text variant="bodyLg" tone="secondary" style={{ marginBottom: Spacing.five }}>
        Design system — Neue Haas Grotesk Display, warm neutrals, one accent.
      </Text>

      <Text variant="caption" tone="tertiary" style={styles.label}>
        Type scale
      </Text>
      <Heading level="title">Title</Heading>
      <Heading level="heading">Heading</Heading>
      <Text variant="bodyLg">Body large — the lede.</Text>
      <Text variant="body">Body — comfortable reading copy at 17px.</Text>
      <Text variant="small" tone="secondary">
        Small — secondary metadata.
      </Text>

      <Text variant="caption" tone="tertiary" style={styles.label}>
        Color
      </Text>
      <View style={styles.swatchRow}>
        {swatches.map((s) => (
          <View key={s.name} style={styles.swatch}>
            <View style={[styles.swatchBox, { backgroundColor: s.color, borderColor: theme.border }]} />
            <Text variant="small">{s.name}</Text>
          </View>
        ))}
      </View>

      <Text variant="caption" tone="tertiary" style={styles.label}>
        Buttons
      </Text>
      <View style={{ gap: Spacing.two }}>
        <Button variant="primary">Primary</Button>
        <Button variant="accent">Pay now</Button>
        <Button variant="ghost">Ghost</Button>
      </View>

      <Text variant="caption" tone="tertiary" style={styles.label}>
        Input
      </Text>
      <View style={{ gap: Spacing.three }}>
        <Input label="Deposit amount" placeholder="0,00" hint="Minimum ₺50,00" keyboardType="decimal-pad" />
        <Input label="Email" placeholder="you@example.com" keyboardType="email-address" />
      </View>

      <Text variant="caption" tone="tertiary" style={styles.label}>
        Surface & Amount
      </Text>
      <View style={{ gap: Spacing.three }}>
        <Surface>
          <Text variant="caption" tone="tertiary">
            Available credit
          </Text>
          <Amount minorUnits={123456} size="display" />
        </Surface>
        <Surface elevated>
          <Text variant="caption" tone="tertiary">
            Cloud cost avoided
          </Text>
          <Amount minorUnits={89900} size="display" />
        </Surface>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  content: {
    padding: Spacing.four,
    gap: Spacing.two,
  },
  label: {
    marginTop: Spacing.five,
    marginBottom: Spacing.one,
  },
  swatchRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: Spacing.three,
  },
  swatch: {
    gap: Spacing.one,
  },
  swatchBox: {
    width: 64,
    height: 64,
    borderRadius: 8,
    borderWidth: 1,
  },
});
