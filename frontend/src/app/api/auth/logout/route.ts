import { clearSession } from "@/lib/session";

// POST /api/auth/logout — clear the session cookie.
export async function POST() {
  await clearSession();
  return Response.json({ ok: true });
}
