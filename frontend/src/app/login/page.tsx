"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Heading, Text, Surface, Button, Input } from "@/components/ui";

export default function LoginPage() {
  const router = useRouter();
  const [phone, setPhone] = useState("+905550000002");
  const [code, setCode] = useState("");
  const [step, setStep] = useState<"phone" | "code">("phone");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function sendCode(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    const res = await fetch("/api/auth/start", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone }),
    });
    setLoading(false);
    if (!res.ok) {
      setError("Could not send the code. Try again.");
      return;
    }
    setStep("code");
  }

  async function verifyCode(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    const res = await fetch("/api/auth/verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone, code }),
    });
    setLoading(false);
    if (!res.ok) {
      const d = await res.json().catch(() => ({}));
      setError(d.error ?? "Invalid code.");
      return;
    }
    router.push("/dashboard");
    router.refresh();
  }

  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <Surface elevated className="flex w-full max-w-md flex-col gap-6">
        <div className="flex flex-col gap-2">
          <Text variant="caption" tone="tertiary">
            King Of The North · Merchant
          </Text>
          <Heading level="title">Sign in</Heading>
          <Text variant="small" tone="secondary">
            We send a one-time code to your store&apos;s WhatsApp.
          </Text>
        </div>

        {step === "phone" ? (
          <form onSubmit={sendCode} className="flex flex-col gap-4">
            <Input
              label="WhatsApp number"
              name="phone"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="+90…"
              hint="Demo merchants: +905550000001 · +905550000002"
            />
            <Button type="submit" variant="accent" disabled={loading}>
              {loading ? "Sending…" : "Send code"}
            </Button>
          </form>
        ) : (
          <form onSubmit={verifyCode} className="flex flex-col gap-4">
            <Input
              label="6-digit code"
              name="code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="424242"
              inputMode="numeric"
              hint="Mock mode — the code is 424242"
            />
            <Button type="submit" variant="accent" disabled={loading}>
              {loading ? "Verifying…" : "Verify & continue"}
            </Button>
            <Button type="button" variant="ghost" onClick={() => setStep("phone")}>
              Use a different number
            </Button>
          </form>
        )}

        {error ? (
          <Text variant="small" tone="accent">
            {error}
          </Text>
        ) : null}
      </Surface>
    </div>
  );
}
