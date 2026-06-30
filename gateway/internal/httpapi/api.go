// Package httpapi is the Gateway's REST surface for the apps. It maps JSON requests
// onto Wallet gRPC calls — JSON in, proto across the wire, JSON out — and translates
// gRPC status codes to HTTP codes. No money logic lives here; Wallet is authoritative.
package httpapi

import (
	"encoding/json"
	"net/http"

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

// API holds the dependencies the handlers need.
type API struct {
	wallet walletv1.WalletServiceClient
}

// New builds the API over a Wallet client.
func New(wallet walletv1.WalletServiceClient) *API {
	return &API{wallet: wallet}
}

// Routes registers the REST endpoints on a mux (Go 1.22+ method+path patterns).
func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/deposit", a.deposit)
	mux.HandleFunc("GET /v1/accounts/{id}", a.account)
	mux.HandleFunc("POST /v1/pay", a.pay)
	mux.HandleFunc("POST /v1/node-reward", a.nodeReward)
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
	// A declined payment is a valid 200 response with approved=false, not an error.
	writeJSON(w, http.StatusOK, map[string]any{
		"approved":               resp.GetApproved(),
		"remaining_credit_minor": resp.GetRemainingCreditMinor(),
		"moka_payment_id":        resp.GetMokaPaymentId(),
		"decline_reason":         resp.GetDeclineReason(),
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
