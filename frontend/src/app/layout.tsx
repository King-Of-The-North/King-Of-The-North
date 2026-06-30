import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";

// Neue Haas Grotesk Display — the type voice (see @kotn/design-system).
// Self-hosted via next/font/local; exposed as `--font-display` to Tailwind/@theme.
const neueHaas = localFont({
  variable: "--font-display",
  display: "swap",
  fallback: ["Helvetica Neue", "Helvetica", "Arial", "system-ui", "sans-serif"],
  src: [
    { path: "./fonts/NeueHaasDisplay-Light.ttf", weight: "300", style: "normal" },
    { path: "./fonts/NeueHaasDisplay-Roman.ttf", weight: "400", style: "normal" },
    { path: "./fonts/NeueHaasDisplay-Medium.ttf", weight: "500", style: "normal" },
    { path: "./fonts/NeueHaasDisplay-Bold.ttf", weight: "700", style: "normal" },
    { path: "./fonts/NeueHaasDisplay-Black.ttf", weight: "900", style: "normal" },
  ],
});

export const metadata: Metadata = {
  title: "King Of The North — Store & Admin",
  description:
    "Merchant catalog and operator dashboard for the King Of The North payment system.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${neueHaas.variable} h-full antialiased`}>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
