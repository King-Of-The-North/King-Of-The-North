import { verifier } from "@/lib/verify";

// POST /api/auth/start { phone } — send a WhatsApp OTP (mock logs the code).
export async function POST(req: Request) {
  const { phone } = (await req.json().catch(() => ({}))) as { phone?: string };
  if (!phone) {
    return Response.json({ error: "phone required" }, { status: 400 });
  }
  await verifier().start(phone);
  return Response.json({ ok: true });
}
