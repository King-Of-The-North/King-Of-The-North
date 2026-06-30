import "server-only";
import { cookies } from "next/headers";
import crypto from "node:crypto";

// Signed session cookie holding the logged-in merchant (ADR-0015). HMAC-signed with
// SESSION_SECRET — demo-grade; a real secret + rotation is required before production.
const COOKIE = "kotn_session";
const SECRET = process.env.SESSION_SECRET ?? "dev-insecure-secret-change-me";
const MAX_AGE = 60 * 60 * 8; // 8h

export type Session = { merchantId: string };

function sign(payload: string): string {
  return crypto.createHmac("sha256", SECRET).update(payload).digest("base64url");
}

function safeEqual(a: string, b: string): boolean {
  const ab = Buffer.from(a);
  const bb = Buffer.from(b);
  return ab.length === bb.length && crypto.timingSafeEqual(ab, bb);
}

export async function createSession(merchantId: string): Promise<void> {
  const payload = Buffer.from(JSON.stringify({ merchantId } satisfies Session)).toString("base64url");
  const token = `${payload}.${sign(payload)}`;
  const jar = await cookies();
  jar.set(COOKIE, token, { httpOnly: true, sameSite: "lax", path: "/", maxAge: MAX_AGE });
}

export async function readSession(): Promise<Session | null> {
  const jar = await cookies();
  const token = jar.get(COOKIE)?.value;
  if (!token) return null;
  const [payload, sig] = token.split(".");
  if (!payload || !sig || !safeEqual(sign(payload), sig)) return null;
  try {
    return JSON.parse(Buffer.from(payload, "base64url").toString()) as Session;
  } catch {
    return null;
  }
}

export async function clearSession(): Promise<void> {
  const jar = await cookies();
  jar.delete(COOKIE);
}
