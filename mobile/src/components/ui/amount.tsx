import { Text as RNText, View, type TextStyle } from 'react-native';
import { formatMinorUnits } from '@kotn/design-system';
import { family, scale, tracking } from '@kotn/design-system';

import { useTheme } from '@/hooks/use-theme';

/**
 * Amount — RN sibling of the web Amount. Integer minor units (kuruş, ADR-0003)
 * via the shared formatter. `fontVariant: tabular-nums` aligns digits. Display only.
 */
type Size = 'heading' | 'title' | 'display' | 'body';

const sizePx: Record<Size, number> = {
  body: scale.body.px,
  heading: scale.heading.px,
  title: scale.title.px,
  display: scale.display.px,
};

export function Amount({
  minorUnits,
  currency = '₺',
  size = 'heading',
}: {
  minorUnits: number | bigint;
  currency?: string;
  size?: Size;
}) {
  const theme = useTheme();
  const fs = sizePx[size];
  const amountStyle: TextStyle = {
    color: theme.text,
    fontFamily: family.native.medium,
    fontSize: fs,
    letterSpacing: parseFloat(tracking.heading) * fs,
    fontVariant: ['tabular-nums'],
  };
  return (
    <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 4 }}>
      <RNText style={{ color: theme.textTertiary, fontFamily: family.native.medium, fontSize: fs * 0.6 }}>
        {currency}
      </RNText>
      <RNText style={amountStyle}>{formatMinorUnits(minorUnits)}</RNText>
    </View>
  );
}
