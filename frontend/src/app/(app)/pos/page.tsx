import { readSession } from "@/lib/session";
import { PosClient } from "./pos-client";

export default async function PosPage() {
  const session = await readSession();
  // Layout guards null sessions; the assertion is safe.
  return <PosClient merchantId={session!.merchantId} />;
}
