"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Text, Button } from "@/components/ui";

const links = [
  { href: "/catalog", label: "Catalog" },
  { href: "/dashboard", label: "Payments" },
  { href: "/admin", label: "Network" },
];

export function Nav({ merchantName }: { merchantName: string }) {
  const pathname = usePathname();
  const router = useRouter();

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.push("/login");
    router.refresh();
  }

  return (
    <header className="flex items-center justify-between border-b border-border px-6 py-4">
      <div className="flex items-center gap-10">
        <Text variant="caption" tone="tertiary">
          KOTN · {merchantName}
        </Text>
        <nav className="flex gap-6">
          {links.map((l) => (
            <Link key={l.href} href={l.href}>
              <Text variant="small" tone={pathname === l.href ? "primary" : "tertiary"}>
                {l.label}
              </Text>
            </Link>
          ))}
        </nav>
      </div>
      <Button variant="ghost" onClick={logout}>
        Sign out
      </Button>
    </header>
  );
}
