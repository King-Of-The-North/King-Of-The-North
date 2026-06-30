/**
 * Money formatting — DISPLAY ONLY. Renders integer minor units (kuruş, ADR-0003)
 * as a grouped major,minor string using integer math (no float drift). Never
 * computes or mutates balances. Shared by web + mobile Amount components.
 */
export function formatMinorUnits(
  minor: number | bigint,
  fractionDigits = 2,
  locale = 'tr-TR',
): string {
  // No BigInt literals (0n/10n) — they require an ES2020 target the web app
  // doesn't set. Use the BigInt() constructor instead.
  const zero = BigInt(0);
  const n = BigInt(minor);
  const negative = n < zero;
  const abs = negative ? -n : n;
  const divisor = BigInt(10) ** BigInt(fractionDigits);
  const major = abs / divisor;
  const frac = abs % divisor;
  const majorStr = Number(major).toLocaleString(locale);
  const fracStr = frac.toString().padStart(fractionDigits, '0');
  return `${negative ? '−' : ''}${majorStr},${fracStr}`;
}
