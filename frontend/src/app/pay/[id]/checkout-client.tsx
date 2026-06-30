"use client";

import { useEffect, useState } from "react";
import QRCode from "qrcode";
import { api, type Charge } from "@/lib/api";
import { Heading, Text, Surface, Button, Input, Amount } from "@/components/ui";

// Public checkout for a payment link. The real secure approval (3DS-equivalent) is the
// on-device face match in the KOTN mobile app — this page hands off (deep link + QR)
// and polls for the result. A labelled demo action stands in until the app exists.
export function CheckoutClient({
  charge: initial,
  merchantName,
}: {
  charge: Charge;
  merchantName: string;
}) {
  const [charge, setCharge] = useState<Charge>(initial);
  const [qr, setQr] = useState("");
  const [customerId, setCustomerId] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const appLink = `kotn://pay/${charge.id}`;
  const isPaid = charge.status === "paid";
  const isOpen = charge.status === "pending";

  useEffect(() => {
    QRCode.toDataURL(appLink, { margin: 1, width: 200 }).then(setQr).catch(() => setQr(""));
  }, [appLink]);

  useEffect(() => {
    if (charge.status !== "pending") return;
    const t = setInterval(async () => {
      try {
        setCharge(await api.getCharge(charge.id));
      } catch {
        /* keep last */
      }
    }, 2000);
    return () => clearInterval(t);
  }, [charge]);

  async function approveDemo() {
    setBusy(true);
    setError("");
    try {
      const res = await api.approveCharge(charge.id, customerId.trim());
      if (!res.approved) setError(res.decline_reason ?? "Declined.");
      setCharge(await api.getCharge(charge.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Approval failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Surface elevated className="flex w-full max-w-md flex-col gap-6">
      <div className="flex flex-col gap-1">
        <Text variant="caption" tone="tertiary">
          Secure payment · {merchantName}
        </Text>
        <Amount minorUnits={charge.amount_minor} size="display" />
      </div>

      <div className="flex flex-col gap-2">
        {charge.items.map((it, i) => (
          <div key={i} className="flex items-center justify-between">
            <Text variant="small" tone="secondary">
              {it.quantity}× {it.name}
            </Text>
            <Amount minorUnits={it.price_minor * it.quantity} size="body" />
          </div>
        ))}
      </div>

      {isPaid ? (
        <Surface className="flex flex-col items-center gap-2 py-6">
          <Heading level="heading">Paid</Heading>
          <Text variant="small" tone="secondary">
            Authenticated on your device. Receipt recorded.
          </Text>
        </Surface>
      ) : isOpen ? (
        <>
          <Surface className="flex flex-col items-center gap-3 py-5">
            {qr ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={qr} alt="Scan with the KOTN app" width={200} height={200} className="rounded-md" />
            ) : null}
            <Text variant="caption" tone="tertiary">
              Strong authentication · on-device face match
            </Text>
            <a href={appLink} className="w-full">
              <Button variant="accent" className="w-full">
                Approve in KOTN app
              </Button>
            </a>
            <Text variant="small" tone="tertiary">
              Scan the code or open the app to confirm with your face.
            </Text>
          </Surface>

          <Surface className="flex flex-col gap-3">
            <Text variant="caption" tone="tertiary">
              Demo — stand in for the mobile approval
            </Text>
            <Input
              label="Customer user id"
              value={customerId}
              onChange={(e) => setCustomerId(e.target.value)}
              placeholder="funded customer UUID"
            />
            <Button variant="primary" onClick={approveDemo} disabled={busy || !customerId}>
              {busy ? "Authenticating…" : "Approve & pay"}
            </Button>
          </Surface>
        </>
      ) : (
        <Text tone="secondary">This payment is {charge.status}.</Text>
      )}

      {error ? (
        <Text variant="small" tone="accent">
          {error}
        </Text>
      ) : null}
    </Surface>
  );
}
