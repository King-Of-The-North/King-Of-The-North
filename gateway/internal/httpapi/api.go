// Package httpapi is the Gateway's REST surface for the apps. It maps JSON requests
// onto Wallet gRPC calls — JSON in, proto across the wire, JSON out — and translates
// gRPC status codes to HTTP codes. No money logic lives here; Wallet is authoritative.
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/ledgerp2p"
	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Fixed-pool defaults (ADR-0001) applied when a deposit omits the Day-Zero params, so
// the app can post just an amount.
const (
	defaultAPY         = 0.12
	defaultCompounding = 12
	defaultLockupYears = 1
	defaultRiskMargin  = 0.10
)

// Ledger is the P2P ledger surface the API needs: a Node plus the cluster's
// replication + DePIN-metering controls (the *ledgerp2p.Cluster satisfies it).
type Ledger interface {
	ledgerp2p.Node
	Nodes() []ledgerp2p.NodeStatus
	KillReplica(id int) bool
	AddReplica(owner string)
	Meter() []ledgerp2p.NodeMeter
	DrainContributions() []ledgerp2p.Contribution
}

// DepinConfig sets the reward economics (ADR-0013). RewardPerEntryMinor must stay
// below CloudCostPerEntryMinor so every payout is funded by real savings and the
// company still nets a margin (reward ≤ value created — never minted from nothing).
type DepinConfig struct {
	RewardPerEntryMinor    int64
	CloudCostPerEntryMinor int64
}

// API holds the dependencies the handlers need.
type API struct {
	wallet walletv1.WalletServiceClient
	ledger Ledger
	depin  DepinConfig

	mu                sync.Mutex
	totalRewardedMin  int64 // lifetime DePIN credit paid out
	totalCloudAvoided int64 // lifetime cloud cost avoided (the value created)
}

// New builds the API over a Wallet client and the P2P ledger cluster.
func New(wallet walletv1.WalletServiceClient, ledger Ledger, depin DepinConfig) *API {
	return &API{wallet: wallet, ledger: ledger, depin: depin}
}

// Routes registers the REST endpoints on a mux (Go 1.22+ method+path patterns).
func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/deposit", a.deposit)
	mux.HandleFunc("GET /v1/accounts/{id}", a.account)
	mux.HandleFunc("POST /v1/pay", a.pay)
	mux.HandleFunc("POST /v1/node-reward", a.nodeReward)
	mux.HandleFunc("GET /v1/ledger", a.ledgerList)
	mux.HandleFunc("GET /v1/ledger/verify", a.ledgerVerify)
	mux.HandleFunc("GET /v1/ledger/pubkey", a.ledgerPubkey)
	mux.HandleFunc("GET /v1/ledger/nodes", a.ledgerNodes)
	mux.HandleFunc("POST /v1/ledger/nodes/{id}/kill", a.ledgerKill)
	mux.HandleFunc("POST /v1/ledger/replicas/add", a.ledgerAddReplica)
	mux.HandleFunc("GET /v1/depin/stats", a.depinStats)
	mux.HandleFunc("POST /v1/depin/settle", a.depinSettle)
}

// --- deposit -> CalculateLimit ---

type depositRequest struct {
	UserID       string  `json:"user_id"`
	DepositMinor int64   `json:"deposit_minor"`
	APY          float64 `json:"apy,omitempty"`
	Compounding  uint32  `json:"compounding_per_year,omitempty"`
	LockupYears  uint32  `json:"lockup_years,omitempty"`
	RiskMargin   float64 `json:"risk_margin,omitempty"`
}

func (a *API) deposit(w http.ResponseWriter, r *http.Request) {
	var req depositRequest
	if !decode(w, r, &req) {
		return
	}
	if req.APY == 0 {
		req.APY = defaultAPY
	}
	if req.Compounding == 0 {
		req.Compounding = defaultCompounding
	}
	if req.LockupYears == 0 {
		req.LockupYears = defaultLockupYears
	}
	if req.RiskMargin == 0 {
		req.RiskMargin = defaultRiskMargin
	}

	resp, err := a.wallet.CalculateLimit(r.Context(), &walletv1.CalculateLimitRequest{
		UserId:             req.UserID,
		DepositMinor:       req.DepositMinor,
		Apy:                req.APY,
		CompoundingPerYear: req.Compounding,
		LockupYears:        req.LockupYears,
		RiskMargin:         req.RiskMargin,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"credit_limit_minor":    resp.GetCreditLimitMinor(),
		"projected_yield_minor": resp.GetProjectedYieldMinor(),
		"lockup_end_date":       resp.GetLockupEndDate(),
	})
}

// --- account -> GetAccount ---

func (a *API) account(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := a.wallet.GetAccount(r.Context(), &walletv1.GetAccountRequest{UserId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":                resp.GetUserId(),
		"principal_minor":        resp.GetPrincipalMinor(),
		"projected_yield_minor":  resp.GetProjectedYieldMinor(),
		"credit_limit_minor":     resp.GetCreditLimitMinor(),
		"available_credit_minor": resp.GetAvailableCreditMinor(),
		"ltv_ratio":              resp.GetLtvRatio(),
		"lockup_end_date":        resp.GetLockupEndDate(),
		"pool_type":              resp.GetPoolType(),
	})
}

// --- pay -> ValidateTransaction ---

type payItem struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceMinor int64  `json:"price_minor"`
	Quantity   uint32 `json:"quantity"`
}

type payRequest struct {
	UserID       string    `json:"user_id"`
	Items        []payItem `json:"items"`
	OtherTrxCode string    `json:"other_trx_code"`
}

func (a *API) pay(w http.ResponseWriter, r *http.Request) {
	var req payRequest
	if !decode(w, r, &req) {
		return
	}
	items := make([]*walletv1.CartItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &walletv1.CartItem{
			Sku:        it.SKU,
			Name:       it.Name,
			PriceMinor: it.PriceMinor,
			Quantity:   it.Quantity,
		})
	}

	resp, err := a.wallet.ValidateTransaction(r.Context(), &walletv1.ValidateTransactionRequest{
		UserId:       req.UserID,
		Items:        items,
		OtherTrxCode: req.OtherTrxCode,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// On approval, append a signed entry to the replicated ledger — the "receipt →
	// entry in the replicated ledger" step (ADR-0005). The money already settled in
	// the authoritative store; this records a tamper-evident audit log. A ledger
	// append failure does not unwind a settled payment — log and continue.
	var ledgerHash string
	if resp.GetApproved() {
		entry, lerr := a.ledger.AppendPayment(
			req.UserID, cartTotal(req.Items), itemLabels(req.Items),
			resp.GetMokaPaymentId(), req.OtherTrxCode)
		if lerr != nil {
			log.Printf("gateway: ledger append failed for %s: %v", req.OtherTrxCode, lerr)
		} else {
			ledgerHash = base64.StdEncoding.EncodeToString(entry.Hash)
		}
	}

	// A declined payment is a valid 200 response with approved=false, not an error.
	writeJSON(w, http.StatusOK, map[string]any{
		"approved":               resp.GetApproved(),
		"remaining_credit_minor": resp.GetRemainingCreditMinor(),
		"moka_payment_id":        resp.GetMokaPaymentId(),
		"decline_reason":         resp.GetDeclineReason(),
		"ledger_hash":            ledgerHash,
	})
}

// cartTotal sums the cart for the ledger record (the wallet is authoritative for the
// deduction; this is the amount we log).
func cartTotal(items []payItem) int64 {
	var total int64
	for _, it := range items {
		total += it.PriceMinor * int64(it.Quantity)
	}
	return total
}

// itemLabels renders the cart as compact human-readable strings for the audit log.
func itemLabels(items []payItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, fmt.Sprintf("%dx %s @%d", it.Quantity, it.Name, it.PriceMinor))
	}
	return out
}

// --- ledger views ---

func (a *API) ledgerList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"length":  a.ledger.Len(),
		"entries": a.ledger.Entries(),
	})
}

func (a *API) ledgerVerify(w http.ResponseWriter, _ *http.Request) {
	if err := a.ledger.Verify(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "length": a.ledger.Len()})
}

func (a *API) ledgerPubkey(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_key": base64.StdEncoding.EncodeToString(a.ledger.PublicKey()),
	})
}

// ledgerNodes shows every node in the cluster — the "you are the infrastructure"
// replication view (anchor + simulated phone replicas).
func (a *API) ledgerNodes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"nodes": a.ledger.Nodes()})
}

// ledgerKill takes a replica offline (demo: kill a phone, lose nothing).
func (a *API) ledgerKill(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id must be an integer"})
		return
	}
	if !a.ledger.KillReplica(id) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no such replica"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"killed": id, "nodes": a.ledger.Nodes()})
}

// ledgerAddReplica brings a new replica online, optionally owned by a user whose phone
// runs it (and therefore earns DePIN rewards). Body: {"owner": "<user_id>"} (optional).
func (a *API) ledgerAddReplica(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Owner string `json:"owner"`
	}
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	a.ledger.AddReplica(body.Owner)
	writeJSON(w, http.StatusOK, map[string]any{"nodes": a.ledger.Nodes()})
}

// --- DePIN metering + rewards (ADR-0008/0013) ---

// depinStats shows per-owned-node contribution, the pending reward, and the lifetime
// "cloud cost avoided" counter (the pitch number).
func (a *API) depinStats(w http.ResponseWriter, _ *http.Request) {
	meters := a.ledger.Meter()
	nodes := make([]map[string]any, 0, len(meters))
	var pendingReward int64
	for _, m := range meters {
		reward := int64(m.Pending) * a.depin.RewardPerEntryMinor
		pendingReward += reward
		nodes = append(nodes, map[string]any{
			"node_id":               m.ID,
			"owner":                 m.Owner,
			"pending_entries":       m.Pending,
			"lifetime_entries":      m.Lifetime,
			"pending_reward_minor":  reward,
			"lifetime_reward_minor": int64(m.Lifetime) * a.depin.RewardPerEntryMinor,
		})
	}

	a.mu.Lock()
	totalRewarded, totalAvoided := a.totalRewardedMin, a.totalCloudAvoided
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"reward_per_entry_minor":     a.depin.RewardPerEntryMinor,
		"cloud_cost_per_entry_minor": a.depin.CloudCostPerEntryMinor,
		"nodes":                      nodes,
		"pending_reward_minor":       pendingReward,
		"total_rewarded_minor":       totalRewarded,
		"total_cloud_avoided_minor":  totalAvoided,
	})
}

// depinSettle pays out pending contributions: for each owned replica it converts
// replicated-entry units into wallet credit via CreditNodeReward (ADR-0008). The
// reward is capped below the cloud cost avoided, so the payout is funded by real
// savings and never minted from nothing (ADR-0013).
func (a *API) depinSettle(w http.ResponseWriter, r *http.Request) {
	contribs := a.ledger.DrainContributions()
	results := make([]map[string]any, 0, len(contribs))
	var paid, avoided int64

	for _, ct := range contribs {
		reward := int64(ct.Units) * a.depin.RewardPerEntryMinor
		if reward <= 0 {
			continue
		}
		ref := fmt.Sprintf("depin-node%d-%d", ct.NodeID, time.Now().UnixNano())
		proof, _ := json.Marshal(struct {
			Minor int64  `json:"minor"`
			Ref   string `json:"ref"`
		}{Minor: reward, Ref: ref})

		resp, err := a.wallet.CreditNodeReward(r.Context(), &walletv1.CreditNodeRewardRequest{
			UserId:            ct.Owner,
			ContributionProof: proof,
		})
		if err != nil {
			// Owner may have no account yet; report and skip (units already drained,
			// so this is best-effort for the demo).
			results = append(results, map[string]any{
				"node_id": ct.NodeID, "owner": ct.Owner, "error": status.Convert(err).Message(),
			})
			continue
		}
		paid += reward
		avoided += int64(ct.Units) * a.depin.CloudCostPerEntryMinor
		results = append(results, map[string]any{
			"node_id":                ct.NodeID,
			"owner":                  ct.Owner,
			"units":                  ct.Units,
			"credited_minor":         resp.GetCreditedMinor(),
			"available_credit_minor": resp.GetAvailableCreditMinor(),
		})
	}

	a.mu.Lock()
	a.totalRewardedMin += paid
	a.totalCloudAvoided += avoided
	totalRewarded, totalAvoided := a.totalRewardedMin, a.totalCloudAvoided
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"settled":                   results,
		"paid_now_minor":            paid,
		"cloud_avoided_now_minor":   avoided,
		"total_rewarded_minor":      totalRewarded,
		"total_cloud_avoided_minor": totalAvoided,
	})
}

// --- node-reward -> CreditNodeReward ---

// nodeRewardRequest is the REST shape; the Gateway packs {minor, ref} into the
// contribution_proof bytes the Wallet expects (the Gateway is the metering authority
// that decides the amount — ADR-0008/0013).
type nodeRewardRequest struct {
	UserID string `json:"user_id"`
	Minor  int64  `json:"minor"`
	Ref    string `json:"ref"`
}

func (a *API) nodeReward(w http.ResponseWriter, r *http.Request) {
	var req nodeRewardRequest
	if !decode(w, r, &req) {
		return
	}
	proof, _ := json.Marshal(struct {
		Minor int64  `json:"minor"`
		Ref   string `json:"ref"`
	}{Minor: req.Minor, Ref: req.Ref})

	resp, err := a.wallet.CreditNodeReward(r.Context(), &walletv1.CreditNodeRewardRequest{
		UserId:            req.UserID,
		ContributionProof: proof,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"credited_minor":         resp.GetCreditedMinor(),
		"available_credit_minor": resp.GetAvailableCreditMinor(),
	})
}

// --- helpers ---

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// writeGRPCError maps a gRPC status onto an HTTP status + JSON error body.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	code := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.Unavailable:
		code = http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		code = http.StatusGatewayTimeout
	}
	writeJSON(w, code, map[string]any{"error": st.Message()})
}
