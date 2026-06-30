# @kotn/design-system

Shared, cross-platform design tokens + fonts for King Of The North (`frontend/` web + `mobile/` Expo).
One TS source of truth → JS objects for React Native, generated CSS custom properties for Tailwind v4.
Aesthetic target: editorial-brutalist (warm neutrals, bold grotesk display, single accent) per
`docs` top-design rubric.

## Layout

```
src/tokens/   colors · typography · spacing · radius · motion   (the source of truth)
src/css/      generate.ts  → emits frontend/src/app/tokens.css
fonts/        Neue Haas Grotesk Display subset (Light/Roman/Medium/Bold/Black)
```

## Usage

- **Mobile / any JS:** `import { semantic, scale, spacing, easing } from '@kotn/design-system/tokens'`
- **Web CSS:** vars come from the generated `tokens.css`; regenerate with
  `pnpm --filter @kotn/design-system build:css`.
- **Re-brand:** change `raw.accent` in `src/tokens/colors.ts`, regenerate CSS — done.

## ⚠️ Font license

`fonts/` holds **Neue Haas Grotesk Display** (commercial Linotype face) sourced from cdnfonts —
**not licensed for production**. Fine for the hackathon demo. For production either license it or
swap to a free grotesk (Space Grotesk / General Sans): change `family.native` / the web `@font-face`
and the font files only — no token or component changes needed.
