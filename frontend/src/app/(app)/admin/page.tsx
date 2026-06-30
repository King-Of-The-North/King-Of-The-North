"use client";

import { useEffect, useState } from "react";
import { api, type LedgerNode, type DepinStats } from "@/lib/api";
import { Heading, Text, Surface, Amount } from "@/components/ui";

export default function AdminPage() {
  const [nodes, setNodes] = useState<LedgerNode[]>([]);
  const [stats, setStats] = useState<DepinStats | null>(null);
  const [chain, setChain] = useState<{ valid: boolean; length: number } | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    let alive = true;
    async function tick() {
      try {
        const [n, s, v] = await Promise.all([api.ledgerNodes(), api.depinStats(), api.ledgerVerify()]);
        if (!alive) return;
        setNodes(n);
        setStats(s);
        setChain(v);
        setErr("");
      } catch (e) {
        if (alive) setErr(e instanceof Error ? e.message : "fetch failed");
      }
    }
    tick();
    const t = setInterval(tick, 3000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);

  return (
    <>
      <div className="flex flex-col gap-2">
        <Text variant="caption" tone="tertiary">
          DePIN network · live
        </Text>
        <Heading level="title">Network</Heading>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Surface elevated className="flex flex-col gap-2">
          <Text variant="caption" tone="tertiary">
            Cloud cost avoided
          </Text>
          <Amount minorUnits={stats?.total_cloud_avoided_minor ?? 0} size="display" />
          <Text variant="small" tone="secondary">
            infrastructure the crowd replaced
          </Text>
        </Surface>
        <Surface elevated className="flex flex-col gap-2">
          <Text variant="caption" tone="tertiary">
            Paid to node runners
          </Text>
          <Amount minorUnits={stats?.total_rewarded_minor ?? 0} size="display" />
          <Text variant="small" tone="secondary">
            credit earned by phones
          </Text>
        </Surface>
      </div>

      <Surface className="flex items-center justify-between">
        <Text variant="caption" tone="tertiary">
          Ledger integrity
        </Text>
        <Text variant="small" tone={chain?.valid ? "primary" : "accent"}>
          {chain ? (chain.valid ? `verified · ${chain.length} entries` : "TAMPERED") : "…"}
        </Text>
      </Surface>

      <div className="flex flex-col gap-2">
        <Text variant="caption" tone="tertiary">
          Nodes ({nodes.length})
        </Text>
        {nodes.map((n) => (
          <Surface key={`${n.role}-${n.id}`} className="flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <Text variant="small">
                {n.role === "anchor" ? "Anchor" : `Replica #${n.id}`}
                {n.owner ? ` · owner ${n.owner.slice(0, 8)}` : ""}
              </Text>
              <Text variant="caption" tone="tertiary">
                {n.length} entries
              </Text>
            </div>
            <Text variant="caption" tone={n.in_sync && n.verified ? "primary" : "accent"}>
              {n.in_sync && n.verified ? "in sync" : "out of sync"}
            </Text>
          </Surface>
        ))}
      </div>

      {err ? (
        <Text variant="small" tone="accent">
          {err}
        </Text>
      ) : null}
    </>
  );
}
