import { useFonts } from 'expo-font';
import { DarkTheme, DefaultTheme, ThemeProvider } from 'expo-router';
import { useColorScheme } from 'react-native';

import { AnimatedSplashOverlay } from '@/components/animated-icon';
import AppTabs from '@/components/app-tabs';
import { FontAssets } from '@/constants/theme';

export default function TabLayout() {
  const colorScheme = useColorScheme();
  // Self-host Neue Haas Grotesk Display (the type voice). Keep the splash overlay
  // up until the faces are ready so we never flash the system fallback.
  const [fontsLoaded] = useFonts(FontAssets);

  return (
    <ThemeProvider value={colorScheme === 'dark' ? DarkTheme : DefaultTheme}>
      <AnimatedSplashOverlay />
      {fontsLoaded ? <AppTabs /> : null}
    </ThemeProvider>
  );
}
