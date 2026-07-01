import Link from "next/link";
import { readSession } from "@/lib/session";
import { serverMerchantCharges } from "@/lib/server-api";
import { Heading, Text, Surface, Button, Amount } from "@/components/ui";
import type { Charge, ChargeStatus } from "@/lib/api";

export const dynamic = "force-dynamic";

const statusTone: Record<ChargeStatus, "primary" | "secondary" | "tertiary"> = {
  paid: "primary",
  pending: "secondary",
  canceled: "tertiary",
  expired: "tertiary",
};

export default async function DashboardPage() {
  const session = await readSession();
  const charges: Charge[] = session ? await serverMerchantCharges(session.merchantId) : [];
  charges.sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at));

  const paid = charges.filter((c) => c.status === "paid");
  const collected = paid.reduce((sum, c) => sum + c.amount_minor, 0);

  return (
    <>
      <div className="flex items-end justify-between">
        <div className="flex flex-col gap-2">
          <Text variant="caption" tone="tertiary">
            Store
          </Text>
          <Heading level="title">Payments</Heading>
        </div>
        <Link href="/catalog">
          <Button variant="ghost">Manage catalog</Button>
        </Link>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Surface elevated className="flex flex-col gap-2">
          <Text variant="caption" tone="tertiary">
            Collected
          </Text>
          <Amount minorUnits={collected} size="display" />
        </Surface>
        <Surface elevated className="flex flex-col gap-2">
          <Text variant="caption" tone="tertiary">
            Paid / total
          </Text>
          <Heading level="title">
            {paid.length} / {charges.length}
          </Heading>
        </Surface>
      </div>

      {charges.length === 0 ? (
        <Surface className="flex flex-col items-center gap-2 py-12">
          <Text tone="secondary">No payments yet.</Text>
          <Link href="/catalog">
            <Button variant="ghost">Set up your catalog</Button>
          </Link>
        </Surface>
      ) : (
        <div className="flex flex-col gap-3">
          {charges.map((c) => (
            <Surface key={c.id} className="flex items-center justify-between">
              <div className="flex flex-col gap-1">
                <Text variant="small" tone="secondary">
                  {c.items.map((it) => `${it.quantity}× ${it.name}`).join(", ") || "—"}
                </Text>
                <Text variant="caption" tone="tertiary">
                  {c.id.slice(0, 12)} · {new Date(c.created_at).toLocaleString()}
                </Text>
              </div>
              <div className="flex items-center gap-6">
                <Text variant="caption" tone={statusTone[c.status]}>
                  {c.status}
                </Text>
                <Amount minorUnits={c.amount_minor} size="heading" />
              </div>
            </Surface>
          ))}
        </div>
      )}
    </>
  );
}
