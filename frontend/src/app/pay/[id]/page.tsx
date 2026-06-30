import { serverCharge, serverMerchants } from "@/lib/server-api";
import { Heading, Text, Surface } from "@/components/ui";
import { CheckoutClient } from "./checkout-client";

export const dynamic = "force-dynamic";

// Public hosted checkout for a payment link (ADR-0016). No merchant auth — this is what
// a customer opens. The 3D-Secure-style approval happens in the KOTN mobile app.
export default async function CheckoutPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const charge = await serverCharge(id);

  if (!charge) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <Surface elevated className="flex max-w-md flex-col gap-2">
          <Heading level="title">Link not found</Heading>
          <Text tone="secondary">This payment link is invalid or has expired.</Text>
        </Surface>
      </div>
    );
  }

  const merchant = (await serverMerchants()).find((m) => m.id === charge.merchant_id);

  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <CheckoutClient charge={charge} merchantName={merchant?.name ?? charge.merchant_id} />
    </div>
  );
}
