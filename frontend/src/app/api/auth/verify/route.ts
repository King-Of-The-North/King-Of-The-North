import { verifier } from "@/lib/verify";
import { serverMerchants } from "@/lib/server-api";
import { createSession } from "@/lib/session";

// POST /api/auth/verify { phone, code } — check the OTP, map the phone to a seeded
// merchant, and set the session cookie (ADR-0015).
export async function POST(req: Request) {
  const { phone, code } = (await req.json().catch(() => ({}))) as {
    phone?: string;
    code?: string;
  };
  if (!phone || !code) {
    return Response.json({ error: "phone and code required" }, { status: 400 });
  }
  if (!(await verifier().check(phone, code))) {
    return Response.json({ error: "invalid code" }, { status: 401 });
  }
  const merchant = (await serverMerchants()).find((m) => m.phone === phone);
  if (!merchant) {
    return Response.json({ error: "no merchant registered for this phone" }, { status: 404 });
  }
  await createSession(merchant.id);
  return Response.json({ ok: true, merchant });
}
