"use client";

import { useEffect, useState } from "react";
import QRCode from "qrcode";
import { api, type Charge, type ChargeItem } from "@/lib/api";
import { Heading, Text, Surface, Button, Input, Amount } from "@/components/ui";

type Draft = { name: string; price: string; quantity: string };
const emptyRow: Draft = { name: "", price: "", quantity: "1" };

// TRY (lira) string → integer kuruş for the API (input edge only; the gateway/wallet
// stay authoritative for money — ADR-0003).
function toMinor(lira: string): number {
  return Math.round(parseFloat(lira || "0") * 100);
}

function webOrigin(): string {
  return process.env.NEXT_PUBLIC_WEB_URL ?? (typeof window !== "undefined" ? window.location.origin : "");
}

export function PosClient({ merchantId }: { merchantId: string }) {
  const [rows, setRows] = useState<Draft[]>([{ ...emptyRow }]);
  const [charge, setCharge] = useState<Charge | null>(null);
  const [qr, setQr] = useState("");
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const payLink = charge ? `${webOrigin()}/pay/${charge.id}` : "";

  // Poll while pending so the merchant sees it flip to paid live.
  useEffect(() => {
    if (!charge || charge.status !== "pending") return;
    const t = setInterval(async () => {
      try {
        setCharge(await api.getCharge(charge.id));
      } catch {
        /* keep last */
      }
    }, 2000);
    return () => clearInterval(t);
  }, [charge]);

  // QR of the shareable payment link (scanning opens the hosted checkout).
  useEffect(() => {
    if (!charge) return;
    const link = `${webOrigin()}/pay/${charge.id}`;
    QRCode.toDataURL(link, { margin: 1, width: 200 }).then(setQr).catch(() => setQr(""));
  }, [charge]);

  function updateRow(i: number, patch: Partial<Draft>) {
    setRows((rs) => rs.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }

  async function createCharge() {
    setError("");
    const items: ChargeItem[] = rows
      .filter((r) => r.name && toMinor(r.price) > 0)
      .map((r) => ({
        sku: r.name.toUpperCase().replace(/\s+/g, "-"),
        name: r.name,
        price_minor: toMinor(r.price),
        quantity: Math.max(1, parseInt(r.quantity || "1", 10)),
      }));
    if (items.length === 0) {
      setError("Add at least one item with a price.");
      return;
    }
    setBusy(true);
    try {
      setCharge(await api.createCharge(merchantId, items));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not create the charge.");
    } finally {
      setBusy(false);
    }
  }

  function reset() {
    setCharge(null);
    setRows([{ ...emptyRow }]);
    setError("");
    setCopied(false);
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(payLink);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard blocked — link is still visible to copy by hand */
    }
  }

  if (charge) {
    const isPaid = charge.status === "paid";
    return (
      <>
        <Heading level="title">{isPaid ? "Paid" : "Payment link"}</Heading>
        <Surface elevated className="flex flex-col items-center gap-5 py-8">
          <Amount minorUnits={charge.amount_minor} size="display" />
          <Text variant="caption" tone={isPaid ? "primary" : "secondary"}>
            {charge.status}
          </Text>
          {!isPaid && qr ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={qr} alt="Payment link QR" width={200} height={200} className="rounded-md" />
          ) : null}
        </Surface>

        {!isPaid ? (
          <Surface className="flex flex-col gap-3">
            <Text variant="caption" tone="tertiary">
              Send this link to your customer
            </Text>
            <div className="flex items-center gap-3">
              <Text variant="small" tone="secondary" className="flex-1 break-all">
                {payLink}
              </Text>
              <Button variant="primary" onClick={copyLink}>
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <Text variant="small" tone="tertiary">
              They approve with their face in the KOTN app — this page updates to paid.
            </Text>
          </Surface>
        ) : (
          <Surface className="flex flex-col gap-1">
            <Text variant="small" tone="secondary">
              Settled · {charge.customer_id?.slice(0, 12)} · {charge.moka_ref}
            </Text>
          </Surface>
        )}

        {error ? (
          <Text variant="small" tone="accent">
            {error}
          </Text>
        ) : null}

        <Button variant="ghost" onClick={reset}>
          New payment link
        </Button>
      </>
    );
  }

  return (
    <>
      <div className="flex flex-col gap-2">
        <Text variant="caption" tone="tertiary">
          Online POS
        </Text>
        <Heading level="title">New payment link</Heading>
      </div>

      <Surface elevated className="flex flex-col gap-4">
        {rows.map((r, i) => (
          <div key={i} className="grid grid-cols-[1fr_120px_90px] gap-3">
            <Input
              label={i === 0 ? "Item" : undefined}
              value={r.name}
              onChange={(e) => updateRow(i, { name: e.target.value })}
              placeholder="Coffee"
            />
            <Input
              label={i === 0 ? "Price ₺" : undefined}
              value={r.price}
              onChange={(e) => updateRow(i, { price: e.target.value })}
              inputMode="decimal"
              placeholder="45.00"
            />
            <Input
              label={i === 0 ? "Qty" : undefined}
              value={r.quantity}
              onChange={(e) => updateRow(i, { quantity: e.target.value })}
              inputMode="numeric"
            />
          </div>
        ))}
        <Button variant="ghost" onClick={() => setRows((rs) => [...rs, { ...emptyRow }])}>
          Add item
        </Button>
      </Surface>

      {error ? (
        <Text variant="small" tone="accent">
          {error}
        </Text>
      ) : null}

      <Button variant="accent" onClick={createCharge} disabled={busy}>
        {busy ? "Creating…" : "Create payment link"}
      </Button>
    </>
  );
}
