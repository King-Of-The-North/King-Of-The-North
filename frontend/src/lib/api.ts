// Typed client for the Gateway REST API (the backend money/POS service). The web app
// never holds money logic — it reads authoritative state from here (frontend/AGENTS.md).
// All amounts are integer minor units (kuruş, ADR-0003); render with <Amount>.

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    cache: "no-store",
  });
  const text = await res.text();
  const body = text ? JSON.parse(text) : {};
  if (!res.ok) {
    throw new ApiError(res.status, body?.error ?? res.statusText);
  }
  return body as T;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// --- types (mirror the gateway JSON shapes) ---

export type Merchant = { id: string; name: string; phone?: string };

export type ChargeStatus = "pending" | "paid" | "canceled" | "expired";

export type ChargeItem = {
  sku: string;
  name: string;
  price_minor: number;
  quantity: number;
};

export type Charge = {
  id: string;
  merchant_id: string;
  amount_minor: number;
  items: ChargeItem[];
  status: ChargeStatus;
  customer_id?: string;
  moka_ref?: string;
  qr_payload: string;
  created_at: string;
  expires_at: string;
};

export type Account = {
  user_id: string;
  principal_minor: number;
  projected_yield_minor: number;
  credit_limit_minor: number;
  available_credit_minor: number;
  ltv_ratio: number;
  lockup_end_date: string;
  pool_type: string;
};

export type LedgerNode = {
  id: number;
  owner?: string;
  role: "anchor" | "replica";
  length: number;
  in_sync: boolean;
  verified: boolean;
};

export type DepinStats = {
  reward_per_entry_minor: number;
  cloud_cost_per_entry_minor: number;
  nodes: {
    node_id: number;
    owner: string;
    pending_entries: number;
    lifetime_entries: number;
    pending_reward_minor: number;
    lifetime_reward_minor: number;
  }[];
  pending_reward_minor: number;
  total_rewarded_minor: number;
  total_cloud_avoided_minor: number;
};

// --- endpoints ---

export const api = {
  // merchants / online POS (ADR-0014)
  merchants: () => req<{ merchants: Merchant[] }>("/v1/merchants").then((r) => r.merchants),

  merchantCharges: (merchantId: string) =>
    req<{ charges: Charge[] }>(`/v1/merchants/${merchantId}/charges`).then((r) => r.charges ?? []),

  createCharge: (merchantId: string, items: ChargeItem[]) =>
    req<Charge>("/v1/charges", {
      method: "POST",
      body: JSON.stringify({ merchant_id: merchantId, items }),
    }),

  getCharge: (id: string) => req<Charge>(`/v1/charges/${id}`),

  approveCharge: (id: string, userId: string) =>
    req<Charge & { approved: boolean; decline_reason?: string; remaining_credit_minor?: number }>(
      `/v1/charges/${id}/approve`,
      { method: "POST", body: JSON.stringify({ user_id: userId }) },
    ),

  cancelCharge: (id: string) => req<Charge>(`/v1/charges/${id}/cancel`, { method: "POST" }),

  // wallet
  account: (userId: string) => req<Account>(`/v1/accounts/${userId}`),

  // admin / DePIN
  ledgerNodes: () => req<{ nodes: LedgerNode[] }>("/v1/ledger/nodes").then((r) => r.nodes),
  ledgerVerify: () => req<{ valid: boolean; length: number }>("/v1/ledger/verify"),
  depinStats: () => req<DepinStats>("/v1/depin/stats"),
};
