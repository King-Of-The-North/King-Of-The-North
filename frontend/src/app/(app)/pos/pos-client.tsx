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

export function PosClient({ merchantId }: { merchantId: string }) {
  const [rows, setRows] = useState<Draft[]>([{ ...emptyRow }]);
  const [charge, setCharge] = useState<Charge | null>(null);
  const [qr, setQr] = useState<string>("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [customerId, setCustomerId] = useState("");

  // Poll the charge while it is pending so the operator sees it flip to paid live.
  useEffect(() => {
    if (!charge || charge.status !== "pending") return;
    const t = setInterval(async () => {
      try {
        setCharge(await api.getCharge(charge.id));
      } catch {
        /* keep last state */
      }
    }, 2000);
    return () => clearInterval(t);
  }, [charge]);

  // Render the QR whenever a charge is created. (No charge → the form view is shown,
  // which doesn't read qr, so there's nothing to clear.)
  useEffect(() => {
    if (!charge) return;
    QRCode.toDataURL(charge.qr_payload, { margin: 1, width: 220 }).then(setQr).catch(() => setQr(""));
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
    setCustomerId("");
    setError("");
  }

  async function approveAsCustomer() {
    if (!charge) return;
    setError("");
    setBusy(true);
    try {
      const res = await api.approveCharge(charge.id, customerId.trim());
      if (!res.approved) {
        setError(res.decline_reason ?? "Declined.");
      }
      setCharge(await api.getCharge(charge.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Approval failed.");
    } finally {
      setBusy(false);
    }
  }

  if (charge) {
    const isPaid = charge.status === "paid";
    return (
      <>
        <Heading level="title">{isPaid ? "Paid" : "Waiting for customer"}</Heading>
        <Surface elevated className="flex flex-col items-center gap-5 py-8">
          <Amount minorUnits={charge.amount_minor} size="display" />
          <Text variant="caption" tone={isPaid ? "primary" : "secondary"}>
            {charge.status}
          </Text>
          {!isPaid && qr ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={qr} alt="Scan to pay" width={220} height={220} className="rounded-md" />
          ) : null}
          {!isPaid ? (
            <Text variant="small" tone="tertiary">
              Customer scans this in the KOTN app to pay.
            </Text>
          ) : (
            <Text variant="small" tone="secondary">
              Settled · {charge.customer_id?.slice(0, 12)} · {charge.moka_ref}
            </Text>
          )}
        </Surface>

        {!isPaid ? (
          <Surface className="flex flex-col gap-3">
            <Text variant="caption" tone="tertiary">
              Demo — approve as a customer
            </Text>
            <Input
              label="Customer user id"
              value={customerId}
              onChange={(e) => setCustomerId(e.target.value)}
              placeholder="customer UUID with a funded wallet"
            />
            <Button variant="primary" onClick={approveAsCustomer} disabled={busy || !customerId}>
              {busy ? "Approving…" : "Approve & settle"}
            </Button>
          </Surface>
        ) : null}

        {error ? (
          <Text variant="small" tone="accent">
            {error}
          </Text>
        ) : null}

        <Button variant="ghost" onClick={reset}>
          New charge
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
        <Heading level="title">New charge</Heading>
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
        {busy ? "Creating…" : "Create charge & show QR"}
      </Button>
    </>
  );
}
