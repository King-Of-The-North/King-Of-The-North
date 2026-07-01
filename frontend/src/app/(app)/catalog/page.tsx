import { readSession } from "@/lib/session";
import { CatalogClient } from "./catalog-client";

export default async function CatalogPage() {
  const session = await readSession();
  return <CatalogClient merchantId={session!.merchantId} />;
}
