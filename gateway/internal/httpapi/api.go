// Package httpapi is the Gateway's REST surface for the apps. It maps JSON requests
// onto Wallet gRPC calls — JSON in, proto across the wire, JSON out — and translates
// gRPC status codes to HTTP codes. No money logic lives here; Wallet is authoritative.
package httpapi

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/catalog"
	"github.com/king-of-the-north/king-of-the-north/gateway/internal/charges"
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
	AddWSReplica(owner string) (int, []ledgerp2p.Entry, chan ledgerp2p.Entry)
	AckWSReplica(id, seq int)
	Meter() []ledgerp2p.NodeMeter
	ClearPending(id, units int)
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
	wallet  walletv1.WalletServiceClient
	ledger  Ledger
	charges *charges.Store
	catalog *catalog.Store
	depin   DepinConfig

	mu                sync.Mutex
	totalRewardedMin  int64 // lifetime DePIN credit paid out
	totalCloudAvoided int64 // lifetime cloud cost avoided (the value created)
}

// New builds the API over a Wallet client, the P2P ledger cluster, the online-POS
// charge store, and the merchant catalog. The device registry now lives in the wallet
// (Postgres), reached through the wallet client.
func New(wallet walletv1.WalletServiceClient, ledger Ledger, store *charges.Store, cat *catalog.Store, depin DepinConfig) *API {
	return &API{wallet: wallet, ledger: ledger, charges: store, catalog: cat, depin: depin}
}

// Routes registers the REST endpoints on a mux (Go 1.22+ method+path patterns).
func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/deposit", a.deposit)
	mux.HandleFunc("GET /v1/accounts/{id}", a.account)
	mux.HandleFunc("GET /v1/accounts/{id}/transactions", a.accountTransactions)
	mux.HandleFunc("POST /v1/pay", a.pay)
	mux.HandleFunc("POST /v1/pay/signed", a.paySigned)
	mux.HandleFunc("POST /v1/transfer", a.transfer)
	mux.HandleFunc("POST /v1/node-reward", a.nodeReward)
	mux.HandleFunc("GET /v1/ledger", a.ledgerList)
	mux.HandleFunc("GET /v1/ledger/verify", a.ledgerVerify)
	mux.HandleFunc("GET /v1/ledger/pubkey", a.ledgerPubkey)
	mux.HandleFunc("GET /v1/ledger/nodes", a.ledgerNodes)
	mux.HandleFunc("POST /v1/ledger/nodes/{id}/kill", a.ledgerKill)
	mux.HandleFunc("POST /v1/ledger/replicas/add", a.ledgerAddReplica)
	mux.HandleFunc("GET /v1/depin/stats", a.depinStats)
	mux.HandleFunc("GET /v1/depin/earnings/{user}", a.depinEarnings)
	mux.HandleFunc("POST /v1/depin/settle", a.depinSettle)
	// Device registry + P2P WebSocket node (phones as real nodes, ADR-0008/0010)
	mux.HandleFunc("POST /v1/devices/enroll", a.deviceEnroll)
	mux.HandleFunc("POST /v1/recovery/rebind", a.recoveryRebind)
	mux.HandleFunc("GET /v1/ledger/ws", a.ledgerWS)
	// Offline spending vouchers (ADR-0012)
	mux.HandleFunc("POST /v1/vouchers", a.voucherIssue)
	mux.HandleFunc("POST /v1/vouchers/{id}/redeem", a.voucherRedeem)
	mux.HandleFunc("POST /v1/vouchers/expire", a.voucherExpire)
	mux.HandleFunc("GET /v1/vouchers/pubkey", a.voucherPubkey)
	mux.HandleFunc("GET /v1/accounts/{id}/vouchers", a.accountVouchers)
	// Online POS (e-commerce charges, ADR-0014)
	mux.HandleFunc("GET /v1/merchants", a.merchantsList)
	mux.HandleFunc("GET /v1/merchants/{id}/charges", a.merchantCharges)
	mux.HandleFunc("POST /v1/charges", a.chargeCreate)
	mux.HandleFunc("GET /v1/charges/{id}", a.chargeGet)
	mux.HandleFunc("POST /v1/charges/{id}/approve", a.chargeApprove)
	mux.HandleFunc("POST /v1/charges/{id}/cancel", a.chargeCancel)
	// Merchant catalog (barcodes → price for scan-and-go, ADR-0007)
	mux.HandleFunc("GET /v1/merchants/{id}/products", a.productsList)
	mux.HandleFunc("POST /v1/merchants/{id}/products", a.productCreate)
	mux.HandleFunc("DELETE /v1/products/{id}", a.productDelete)
	mux.HandleFunc("GET /v1/products/barcode/{barcode}", a.productByBarcode)
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

// --- account transactions -> ListTransactions ---

// accountTransactions returns a user's transactions newest-first for the app's
// receipt/history views. Optional ?limit=N (store caps it).
func (a *API) accountTransactions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var limit uint32
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			limit = uint32(n)
		}
	}
	resp, err := a.wallet.ListTransactions(r.Context(), &walletv1.ListTransactionsRequest{
		UserId: id,
		Limit:  limit,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	txs := make([]map[string]any, 0, len(resp.GetTransactions()))
	for _, t := range resp.GetTransactions() {
		txs = append(txs, map[string]any{
			"id":              t.GetId(),
			"other_trx_code":  t.GetOtherTrxCode(),
			"moka_payment_id": t.GetMokaPaymentId(),
			"amount_minor":    t.GetAmountMinor(),
			"payment_status":  t.GetPaymentStatus(),
			"trx_status":      t.GetTrxStatus(),
			"created_at":      t.GetCreatedAt(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txs})
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
	res, err := a.settle(r.Context(), req.UserID, req.Items, req.OtherTrxCode)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	// A declined payment is a valid 200 response with approved=false, not an error.
	writeJSON(w, http.StatusOK, map[string]any{
		"approved":               res.approved,
		"remaining_credit_minor": res.remaining,
		"moka_payment_id":        res.mokaID,
		"decline_reason":         res.declineReason,
		"ledger_hash":            res.ledgerHash,
	})
}

// --- signed pay (device-authenticated) ---

type paySignedRequest struct {
	UserID       string    `json:"user_id"`
	Items        []payItem `json:"items"`
	OtherTrxCode string    `json:"other_trx_code"`
	DevicePubKey string    `json:"device_pubkey"` // base64-std Ed25519 public key
	Sig          string    `json:"sig"`           // base64-std signature over the canonical cart
}

// canonicalCart is the exact byte string a phone signs to authorize a spend. It is
// deterministic and reproducible on-device:
//
//	"<user_id>|<other_trx_code>|<total_minor>|<sku>:<qty>,<sku>:<qty>,..."
//
// (item order as sent; total via cartTotal). The signature binds the payer, the cart,
// and the idempotency code so it can't be replayed for a different cart.
func canonicalCart(userID, otherTrxCode string, items []payItem) []byte {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%s:%d", it.SKU, it.Quantity))
	}
	return []byte(fmt.Sprintf("%s|%s|%d|%s", userID, otherTrxCode, cartTotal(items), strings.Join(parts, ",")))
}

// paySigned is the on-device face-pay path: the phone signs the cart with its enrolled
// device key, proving an authenticated human authorized this exact spend before any money
// moves. The gateway verifies the signature against the wallet's device registry, then
// runs the same settle() money path as /v1/pay. The plain /v1/pay stays for web POS.
func (a *API) paySigned(w http.ResponseWriter, r *http.Request) {
	var req paySignedRequest
	if !decode(w, r, &req) {
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.DevicePubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "device_pubkey must be a base64 Ed25519 key"})
		return
	}
	sig, err := base64.StdEncoding.DecodeString(req.Sig)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "sig must be base64"})
		return
	}
	if !a.deviceEnrolled(r.Context(), req.UserID, pub) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "device not enrolled"})
		return
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), canonicalCart(req.UserID, req.OtherTrxCode, req.Items), sig) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "bad signature"})
		return
	}

	res, err := a.settle(r.Context(), req.UserID, req.Items, req.OtherTrxCode)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"approved":               res.approved,
		"remaining_credit_minor": res.remaining,
		"moka_payment_id":        res.mokaID,
		"decline_reason":         res.declineReason,
		"ledger_hash":            res.ledgerHash,
	})
}

// --- transfer -> Transfer ---

type transferRequest struct {
	FromUserID  string `json:"from_user_id"`
	ToUserID    string `json:"to_user_id"`
	AmountMinor int64  `json:"amount_minor"`
	Ref         string `json:"ref"`
}

// transfer moves spendable credit user->user. The wallet is authoritative for the
// atomic move; on success we append a signed entry to the replicated P2P ledger
// (ADR-0005) so the transfer is proof-logged and metered like a payment. There is no
// Moka settle — nothing leaves custody. A ledger append failure does not unwind the
// committed transfer (same rule as settle) — log and continue.
func (a *API) transfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := a.wallet.Transfer(r.Context(), &walletv1.TransferRequest{
		FromUserId:  req.FromUserID,
		ToUserId:    req.ToUserID,
		AmountMinor: req.AmountMinor,
		Ref:         req.Ref,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	var ledgerHash string
	entry, lerr := a.ledger.AppendPayment(
		req.FromUserID, req.AmountMinor, []string{"transfer→" + req.ToUserID}, "", req.Ref)
	if lerr != nil {
		log.Printf("gateway: ledger append failed for transfer %s: %v", req.Ref, lerr)
	} else {
		ledgerHash = base64.StdEncoding.EncodeToString(entry.Hash)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from_remaining_minor": resp.GetFromRemainingMinor(),
		"to_available_minor":   resp.GetToAvailableMinor(),
		"from_trx_code":        resp.GetFromTrxCode(),
		"to_trx_code":          resp.GetToTrxCode(),
		"ledger_hash":          ledgerHash,
	})
}

// settleResult is the outcome of settling a cart against the wallet + ledger.
type settleResult struct {
	approved      bool
	remaining     int64
	mokaID        string
	declineReason string
	ledgerHash    string
}

// settle is the one money path shared by customer pay (/v1/pay) and merchant-initiated
// charge approval (/v1/charges/{id}/approve): validate+deduct in the wallet, then on
// approval append a signed ledger entry (ADR-0005). otherTrxCode keys the transaction
// (cart trx code, or the charge id) so it maps 1:1 to a ledger/Moka record.
func (a *API) settle(ctx context.Context, userID string, items []payItem, otherTrxCode string) (settleResult, error) {
	cart := make([]*walletv1.CartItem, 0, len(items))
	for _, it := range items {
		cart = append(cart, &walletv1.CartItem{
			Sku: it.SKU, Name: it.Name, PriceMinor: it.PriceMinor, Quantity: it.Quantity,
		})
	}
	resp, err := a.wallet.ValidateTransaction(ctx, &walletv1.ValidateTransactionRequest{
		UserId:       userID,
		Items:        cart,
		OtherTrxCode: otherTrxCode,
	})
	if err != nil {
		return settleResult{}, err
	}
	res := settleResult{
		approved:      resp.GetApproved(),
		remaining:     resp.GetRemainingCreditMinor(),
		mokaID:        resp.GetMokaPaymentId(),
		declineReason: resp.GetDeclineReason(),
	}
	// A ledger append failure does not unwind a settled payment — log and continue.
	if resp.GetApproved() {
		entry, lerr := a.ledger.AppendPayment(
			userID, cartTotal(items), itemLabels(items), resp.GetMokaPaymentId(), otherTrxCode)
		if lerr != nil {
			log.Printf("gateway: ledger append failed for %s: %v", otherTrxCode, lerr)
		} else {
			res.ledgerHash = base64.StdEncoding.EncodeToString(entry.Hash)
		}
	}
	return res, nil
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
			return
		}
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

// --- device enrollment (P2P WebSocket auth) ---

type enrollRequest struct {
	UserID       string `json:"user_id"`
	DevicePubKey string `json:"device_pubkey"` // base64-std Ed25519 public key
}

// deviceEnroll binds a phone's Ed25519 device public key to a user (persisted in the
// wallet), so the P2P WebSocket handshake and signed-pay can later prove that a
// connecting phone is that user's device. Only the public key is stored; the private key
// never leaves the phone (ADR-0006/0010). Enrollment is self-asserted for the demo.
func (a *API) deviceEnroll(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if !decode(w, r, &req) {
		return
	}
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user_id required"})
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.DevicePubKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "device_pubkey must be base64"})
		return
	}
	resp, err := a.wallet.EnrollDevice(r.Context(), &walletv1.EnrollDeviceRequest{
		UserId:       req.UserID,
		DevicePubkey: pub,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enrolled":     true,
		"user_id":      req.UserID,
		"device_count": len(resp.GetDevices()),
	})
}

type rebindRequest struct {
	UserID          string `json:"user_id"`
	NewDevicePubKey string `json:"new_device_pubkey"` // base64-std Ed25519 public key
}

// recoveryRebind is account recovery for a lost/new phone (ADR-0011): revoke every
// existing device key and enroll a new one. Money is anchored to user_id and untouched.
// Recovery is self-asserted for the demo — production gates it behind an identity proof.
func (a *API) recoveryRebind(w http.ResponseWriter, r *http.Request) {
	var req rebindRequest
	if !decode(w, r, &req) {
		return
	}
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user_id required"})
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.NewDevicePubKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "new_device_pubkey must be base64"})
		return
	}
	resp, err := a.wallet.RebindDevices(r.Context(), &walletv1.RebindDevicesRequest{
		UserId:          req.UserID,
		NewDevicePubkey: pub,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rebound":      true,
		"user_id":      req.UserID,
		"device_count": len(resp.GetDevices()),
	})
}

// depinEarnings is the per-user view for the app's node/earnings panel: the nodes this
// user runs, entries replicated, pending (unsettled) reward, and lifetime reward. The
// settled reward already lands in the user's wallet balance (CreditNodeReward), so this
// reports the metering side — what the phone has earned by being a server.
func (a *API) depinEarnings(w http.ResponseWriter, r *http.Request) {
	user := r.PathValue("user")
	meters := a.ledger.Meter()
	nodes := make([]map[string]any, 0)
	var pendingEntries, lifetimeEntries int
	for _, m := range meters {
		if m.Owner != user {
			continue
		}
		pendingEntries += m.Pending
		lifetimeEntries += m.Lifetime
		nodes = append(nodes, map[string]any{
			"node_id":          m.ID,
			"pending_entries":  m.Pending,
			"lifetime_entries": m.Lifetime,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":                  user,
		"nodes":                    nodes,
		"node_count":               len(nodes),
		"pending_entries":          pendingEntries,
		"lifetime_entries":         lifetimeEntries,
		"pending_reward_minor":     int64(pendingEntries) * a.depin.RewardPerEntryMinor,
		"lifetime_reward_minor":    int64(lifetimeEntries) * a.depin.RewardPerEntryMinor,
		"cloud_cost_avoided_minor": int64(lifetimeEntries) * a.depin.CloudCostPerEntryMinor,
		"reward_per_entry_minor":   a.depin.RewardPerEntryMinor,
	})
}

// depinSettle pays out pending contributions: for each owned replica it converts
// replicated-entry units into wallet credit via CreditNodeReward (ADR-0008). The
// reward is capped below the cloud cost avoided, so the payout is funded by real
// savings and never minted from nothing (ADR-0013).
// settleDepin credits every owned node's pending contribution to its wallet and clears
// the settled units. Shared by the /v1/depin/settle handler and the auto-settle timer.
func (a *API) settleDepin(ctx context.Context) (paid, avoided int64, results []map[string]any) {
	meters := a.ledger.Meter()
	results = make([]map[string]any, 0, len(meters))

	for _, m := range meters {
		units := m.Pending
		reward := int64(units) * a.depin.RewardPerEntryMinor
		if reward <= 0 {
			continue
		}
		ref := fmt.Sprintf("depin-node%d-%d", m.ID, time.Now().UnixNano())
		proof, _ := json.Marshal(struct {
			Minor int64  `json:"minor"`
			Ref   string `json:"ref"`
		}{Minor: reward, Ref: ref})

		resp, err := a.wallet.CreditNodeReward(ctx, &walletv1.CreditNodeRewardRequest{
			UserId:            m.Owner,
			ContributionProof: proof,
		})
		if err != nil {
			// Credit failed (e.g. owner has no account yet) — leave the units pending
			// so a later settle retries them. Nothing is cleared, nothing is lost.
			results = append(results, map[string]any{
				"node_id": m.ID, "owner": m.Owner, "error": status.Convert(err).Message(),
			})
			continue
		}
		// Clear only the units we just credited (subtract, don't zero) so contributions
		// that accrued during the wallet call survive for the next settle.
		a.ledger.ClearPending(m.ID, units)
		paid += reward
		avoided += int64(units) * a.depin.CloudCostPerEntryMinor
		results = append(results, map[string]any{
			"node_id":                m.ID,
			"owner":                  m.Owner,
			"units":                  units,
			"credited_minor":         resp.GetCreditedMinor(),
			"available_credit_minor": resp.GetAvailableCreditMinor(),
		})
	}

	a.mu.Lock()
	a.totalRewardedMin += paid
	a.totalCloudAvoided += avoided
	a.mu.Unlock()
	return paid, avoided, results
}

// AutoSettle runs one settlement pass for the background timer, returning the amounts
// paid and cloud cost avoided this pass.
func (a *API) AutoSettle(ctx context.Context) (paid, avoided int64) {
	paid, avoided, _ = a.settleDepin(ctx)
	return paid, avoided
}

func (a *API) depinSettle(w http.ResponseWriter, r *http.Request) {
	paid, avoided, results := a.settleDepin(r.Context())
	a.mu.Lock()
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

// --- online POS: merchant-initiated charges (ADR-0014) ---

func (a *API) merchantsList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"merchants": a.charges.Merchants()})
}

func (a *API) merchantCharges(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"charges": a.charges.ChargesForMerchant(r.PathValue("id")),
	})
}

type chargeCreateRequest struct {
	MerchantID string         `json:"merchant_id"`
	Items      []charges.Item `json:"items"`
}

// chargeCreate is the merchant POS action: quote a price as a pending charge with a QR
// for the customer to scan.
func (a *API) chargeCreate(w http.ResponseWriter, r *http.Request) {
	var req chargeCreateRequest
	if !decode(w, r, &req) {
		return
	}
	c, err := a.charges.Create(req.MerchantID, req.Items)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, charges.ErrMerchantNotFound) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeChargeJSON(w, http.StatusCreated, *c, nil)
}

func (a *API) chargeGet(w http.ResponseWriter, r *http.Request) {
	c, err := a.charges.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeChargeJSON(w, http.StatusOK, c, nil)
}

type chargeApproveRequest struct {
	UserID string `json:"user_id"`
}

// chargeApprove is the customer action (after on-device face match): settle the charge
// against their wallet. Reuses the one money path (settle); on approval the charge is
// marked paid. A declined or already-settled charge is reported, not charged twice.
func (a *API) chargeApprove(w http.ResponseWriter, r *http.Request) {
	var req chargeApproveRequest
	if !decode(w, r, &req) {
		return
	}
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user_id required"})
		return
	}

	id := r.PathValue("id")
	c, err := a.charges.BeginApprove(id)
	if err != nil {
		code := http.StatusConflict
		if errors.Is(err, charges.ErrChargeNotFound) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}

	res, err := a.settle(r.Context(), req.UserID, chargeItemsToPay(c.Items), id)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !res.approved {
		writeChargeJSON(w, http.StatusOK, c, map[string]any{
			"approved": false, "decline_reason": res.declineReason,
		})
		return
	}

	paid, err := a.charges.MarkPaid(id, req.UserID, res.mokaID)
	if err != nil {
		// Settled in the wallet but the charge was finalized concurrently — surface it.
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeChargeJSON(w, http.StatusOK, paid, map[string]any{
		"approved":               true,
		"remaining_credit_minor": res.remaining,
		"ledger_hash":            res.ledgerHash,
	})
}

func (a *API) chargeCancel(w http.ResponseWriter, r *http.Request) {
	c, err := a.charges.Cancel(r.PathValue("id"))
	if err != nil {
		code := http.StatusConflict
		if errors.Is(err, charges.ErrChargeNotFound) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeChargeJSON(w, http.StatusOK, c, nil)
}

// writeChargeJSON renders a charge with its QR payload, merging optional extra fields.
func writeChargeJSON(w http.ResponseWriter, code int, c charges.Charge, extra map[string]any) {
	body := map[string]any{
		"id":           c.ID,
		"merchant_id":  c.MerchantID,
		"amount_minor": c.AmountMinor,
		"items":        c.Items,
		"status":       c.Status,
		"customer_id":  c.CustomerID,
		"moka_ref":     c.MokaRef,
		"qr_payload":   c.QRPayload(),
		"created_at":   c.CreatedAt,
		"expires_at":   c.ExpiresAt,
	}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, code, body)
}

func chargeItemsToPay(items []charges.Item) []payItem {
	out := make([]payItem, 0, len(items))
	for _, it := range items {
		out = append(out, payItem{SKU: it.SKU, Name: it.Name, PriceMinor: it.PriceMinor, Quantity: it.Quantity})
	}
	return out
}

// --- merchant catalog (ADR-0007) ---

func (a *API) productsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"products": a.catalog.ForMerchant(r.PathValue("id")),
	})
}

type productCreateRequest struct {
	Barcode    string `json:"barcode"`
	Name       string `json:"name"`
	PriceMinor int64  `json:"price_minor"`
}

func (a *API) productCreate(w http.ResponseWriter, r *http.Request) {
	var req productCreateRequest
	if !decode(w, r, &req) {
		return
	}
	p, err := a.catalog.Create(r.PathValue("id"), req.Barcode, req.Name, req.PriceMinor)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, catalog.ErrBarcodeTaken) {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) productDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.catalog.Delete(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": r.PathValue("id")})
}

// productByBarcode is the scan-and-go price lookup: a scanned barcode → product.
func (a *API) productByBarcode(w http.ResponseWriter, r *http.Request) {
	p, err := a.catalog.ByBarcode(r.PathValue("barcode"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
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

// writeGRPCError maps a gRPC status onto an HTTP status + JSON error body. Client
// errors (4xx) carry their message through — those are deliberate, safe strings. Server
// errors (5xx) are logged in full but returned generically, so internal details (e.g.
// raw driver messages / SQLSTATE) never leak to the caller.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Printf("gateway: non-status error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "upstream error"})
		return
	}
	code := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.FailedPrecondition:
		code = http.StatusPreconditionFailed
	case codes.AlreadyExists:
		code = http.StatusConflict
	case codes.Unavailable:
		code = http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		code = http.StatusGatewayTimeout
	}
	if code >= 500 {
		log.Printf("gateway: %s: %s", st.Code(), st.Message())
		writeJSON(w, code, map[string]any{"error": "internal error"})
		return
	}
	writeJSON(w, code, map[string]any{"error": st.Message()})
}
