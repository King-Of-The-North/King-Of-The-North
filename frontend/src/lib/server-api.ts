import "server-only";
import type { Merchant } from "./api";

// Server-side Gateway calls (route handlers / server components). Inside Docker the
// browser reaches the gateway on the host (NEXT_PUBLIC_API_URL = localhost:8080) but
// the server reaches it on the internal network — so server code uses API_URL_INTERNAL
// (set to http://gateway:8080 in compose), falling back to the public URL for dev.
const INTERNAL =
  process.env.API_URL_INTERNAL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function serverMerchants(): Promise<Merchant[]> {
  const res = await fetch(`${INTERNAL}/v1/merchants`, { cache: "no-store" });
  if (!res.ok) throw new Error(`merchants fetch failed: ${res.status}`);
  const data = (await res.json()) as { merchants: Merchant[] };
  return data.merchants ?? [];
}

export async function serverMerchantCharges(merchantId: string) {
  const res = await fetch(`${INTERNAL}/v1/merchants/${merchantId}/charges`, { cache: "no-store" });
  if (!res.ok) throw new Error(`charges fetch failed: ${res.status}`);
  const data = await res.json();
  return data.charges ?? [];
}
