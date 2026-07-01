"use client";

import { useEffect, useState } from "react";
import { api, type Product } from "@/lib/api";
import { Heading, Text, Surface, Button, Input, Amount } from "@/components/ui";

// TRY string → integer kuruş (input edge only; money stays authoritative server-side).
function toMinor(lira: string): number {
  return Math.round(parseFloat(lira || "0") * 100);
}

export function CatalogClient({ merchantId }: { merchantId: string }) {
  const [products, setProducts] = useState<Product[]>([]);
  const [name, setName] = useState("");
  const [barcode, setBarcode] = useState("");
  const [price, setPrice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const p = await api.products(merchantId);
        if (alive) setProducts(p);
      } catch (e) {
        if (alive) setError(e instanceof Error ? e.message : "Could not load products.");
      }
    })();
    return () => {
      alive = false;
    };
  }, [merchantId, reloadKey]);

  const reload = () => setReloadKey((k) => k + 1);

  async function add() {
    setError("");
    if (!name || !barcode || toMinor(price) <= 0) {
      setError("Name, barcode and a positive price are required.");
      return;
    }
    setBusy(true);
    try {
      await api.createProduct(merchantId, { barcode, name, price_minor: toMinor(price) });
      setName("");
      setBarcode("");
      setPrice("");
      reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not add the product.");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    await api.deleteProduct(id).catch(() => {});
    reload();
  }

  // Simulate a customer scanning this product: create a charge for it and open the
  // hosted 3DS checkout. In production the mobile scan-and-go does this automatically
  // (ADR-0007) — the owner never creates a link by hand (ADR-0016).
  async function simulateScan(p: Product) {
    setError("");
    try {
      const charge = await api.createCharge(merchantId, [
        { sku: p.barcode, name: p.name, price_minor: p.price_minor, quantity: 1 },
      ]);
      window.open(`/pay/${charge.id}`, "_blank");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not start checkout.");
    }
  }

  return (
    <>
      <div className="flex flex-col gap-2">
        <Text variant="caption" tone="tertiary">
          Store
        </Text>
        <Heading level="title">Catalog</Heading>
        <Text variant="small" tone="secondary">
          Products your customers scan to pay. Scan-and-go reads these by barcode.
        </Text>
      </div>

      <Surface elevated className="flex flex-col gap-4">
        <Text variant="caption" tone="tertiary">
          Add product
        </Text>
        <div className="grid grid-cols-[1fr_1fr_120px] gap-3">
          <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Latte" />
          <Input
            label="Barcode"
            value={barcode}
            onChange={(e) => setBarcode(e.target.value)}
            placeholder="8690000000000"
            inputMode="numeric"
          />
          <Input
            label="Price ₺"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            placeholder="55.00"
            inputMode="decimal"
          />
        </div>
        <Button variant="accent" onClick={add} disabled={busy}>
          {busy ? "Adding…" : "Add to catalog"}
        </Button>
      </Surface>

      {error ? (
        <Text variant="small" tone="accent">
          {error}
        </Text>
      ) : null}

      <div className="flex flex-col gap-3">
        {products.length === 0 ? (
          <Surface className="py-10 text-center">
            <Text tone="secondary">No products yet — add your first above.</Text>
          </Surface>
        ) : (
          products.map((p) => (
            <Surface key={p.id} className="flex items-center justify-between">
              <div className="flex flex-col gap-1">
                <Text variant="small">{p.name}</Text>
                <Text variant="caption" tone="tertiary">
                  {p.barcode}
                </Text>
              </div>
              <div className="flex items-center gap-4">
                <Amount minorUnits={p.price_minor} size="heading" />
                <Button variant="ghost" onClick={() => simulateScan(p)}>
                  Test scan
                </Button>
                <Button variant="ghost" onClick={() => remove(p.id)}>
                  Remove
                </Button>
              </div>
            </Surface>
          ))
        )}
      </div>
    </>
  );
}
