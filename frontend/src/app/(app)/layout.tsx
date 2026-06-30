import { redirect } from "next/navigation";
import { readSession } from "@/lib/session";
import { serverMerchants } from "@/lib/server-api";
import { Nav } from "@/components/nav";

// Guard for the authenticated area: no session → /login. Renders the nav with the
// merchant's name resolved from the session.
export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const session = await readSession();
  if (!session) {
    redirect("/login");
  }
  const merchant = (await serverMerchants()).find((m) => m.id === session.merchantId);
  return (
    <>
      <Nav merchantName={merchant?.name ?? session.merchantId} />
      <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-6 p-6">{children}</main>
    </>
  );
}
