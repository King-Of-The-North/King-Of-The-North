import { Amount, Button, Heading, Input, Surface, Text } from "@/components/ui";

/**
 * Styleguide — the design-system proof harness. Renders every token + primitive.
 * Visit /styleguide to verify fonts, scale, color, states, and motion.
 */
const swatches: { name: string; varName: string }[] = [
  { name: "background", varName: "--kotn-color-background" },
  { name: "surface", varName: "--kotn-color-surface" },
  { name: "surface-elevated", varName: "--kotn-color-surface-elevated" },
  { name: "text-primary", varName: "--kotn-color-text-primary" },
  { name: "text-secondary", varName: "--kotn-color-text-secondary" },
  { name: "accent", varName: "--kotn-color-accent" },
  { name: "border", varName: "--kotn-color-border" },
  { name: "success", varName: "--kotn-color-success" },
  { name: "warning", varName: "--kotn-color-warning" },
  { name: "danger", varName: "--kotn-color-danger" },
];

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={{ marginBottom: "var(--kotn-space-4xl)" }}>
      <Text
        variant="caption"
        tone="tertiary"
        style={{ marginBottom: "var(--kotn-space-lg)", display: "block" }}
      >
        {title}
      </Text>
      {children}
    </section>
  );
}

export default function StyleguidePage() {
  return (
    <main
      style={{
        maxWidth: "var(--kotn-layout-max-content)",
        margin: "0 auto",
        padding: "var(--kotn-space-3xl) var(--kotn-space-lg)",
      }}
    >
      <Heading level="hero" style={{ marginBottom: "var(--kotn-space-2xl)" }}>
        King of the North
      </Heading>
      <Text variant="bodyLg" tone="secondary" style={{ marginBottom: "var(--kotn-space-5xl)" }}>
        Design system — Neue Haas Grotesk Display, warm neutrals, one accent.
      </Text>

      <Section title="Type scale">
        <Heading level="hero">Hero</Heading>
        <Heading level="display">Display</Heading>
        <Heading level="title">Title</Heading>
        <Heading level="heading">Heading</Heading>
        <Text variant="bodyLg">Body large — the lede that draws the reader in.</Text>
        <Text variant="body">
          Body — 17px, generous leading, comfortable measure for reading longer copy.
        </Text>
        <Text variant="small" tone="secondary">
          Small — secondary metadata and supporting detail.
        </Text>
        <Text variant="caption" tone="tertiary">
          Caption / overline
        </Text>
      </Section>

      <Section title="Color">
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))",
            gap: "var(--kotn-space-md)",
          }}
        >
          {swatches.map((s) => (
            <div key={s.name}>
              <div
                style={{
                  height: 72,
                  borderRadius: "var(--kotn-radius-md)",
                  background: `var(${s.varName})`,
                  border: "1px solid var(--kotn-color-border)",
                }}
              />
              <Text variant="small" style={{ marginTop: "var(--kotn-space-sm)" }}>
                {s.name}
              </Text>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Buttons">
        <div style={{ display: "flex", gap: "var(--kotn-space-md)", flexWrap: "wrap" }}>
          <Button variant="primary">Primary</Button>
          <Button variant="accent">Pay now</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="primary" disabled>
            Disabled
          </Button>
        </div>
      </Section>

      <Section title="Input">
        <div style={{ maxWidth: 360, display: "flex", flexDirection: "column", gap: "var(--kotn-space-lg)" }}>
          <Input label="Deposit amount" name="deposit" placeholder="0,00" hint="Minimum ₺50,00" />
          <Input label="Email" name="email" type="email" placeholder="you@example.com" />
        </div>
      </Section>

      <Section title="Surface & Amount">
        <div style={{ display: "flex", gap: "var(--kotn-space-lg)", flexWrap: "wrap" }}>
          <Surface style={{ minWidth: 220 }}>
            <Text variant="caption" tone="tertiary">
              Available credit
            </Text>
            <div style={{ marginTop: "var(--kotn-space-sm)" }}>
              <Amount minorUnits={1_234_56} size="display" />
            </div>
          </Surface>
          <Surface elevated style={{ minWidth: 220 }}>
            <Text variant="caption" tone="tertiary">
              Cloud cost avoided
            </Text>
            <div style={{ marginTop: "var(--kotn-space-sm)" }}>
              <Amount minorUnits={89_900} size="display" />
            </div>
          </Surface>
        </div>
      </Section>
    </main>
  );
}
