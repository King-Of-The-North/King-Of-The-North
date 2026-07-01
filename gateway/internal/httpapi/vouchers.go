package httpapi

import (
	"encoding/base64"
	"log"
	"net/http"

	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
)

// voucherJSON renders a proto Voucher for the app, base64-ing the binary fields.
func voucherJSON(v *walletv1.Voucher) map[string]any {
	return map[string]any{
		"id":              v.GetId(),
		"user_id":         v.GetUserId(),
		"amount_minor":    v.GetAmountMinor(),
		"status":          v.GetStatus(),
		"nonce":           base64.StdEncoding.EncodeToString(v.GetNonce()),
		"sig":             base64.StdEncoding.EncodeToString(v.GetSig()),
		"expires_at":      v.GetExpiresAt(),
		"issued_at":       v.GetIssuedAt(),
		"redeem_trx_code": v.GetRedeemTrxCode(),
	}
}

type voucherIssueRequest struct {
	UserID      string `json:"user_id"`
	AmountMinor int64  `json:"amount_minor"`
	TTLSeconds  uint32 `json:"ttl_seconds"`
}

// voucherIssue reserves credit and returns a wallet-signed offline voucher (ADR-0012).
func (a *API) voucherIssue(w http.ResponseWriter, r *http.Request) {
	var req voucherIssueRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := a.wallet.IssueVoucher(r.Context(), &walletv1.IssueVoucherRequest{
		UserId:      req.UserID,
		AmountMinor: req.AmountMinor,
		TtlSeconds:  req.TTLSeconds,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, voucherJSON(resp))
}

type voucherRedeemRequest struct {
	OtherTrxCode string `json:"other_trx_code"`
}

// voucherRedeem finalizes a voucher once and appends a signed P2P ledger entry for the
// settlement (the credit was already reserved at issue).
func (a *API) voucherRedeem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req voucherRedeemRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := a.wallet.RedeemVoucher(r.Context(), &walletv1.RedeemVoucherRequest{
		VoucherId:    id,
		OtherTrxCode: req.OtherTrxCode,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	var ledgerHash string
	entry, lerr := a.ledger.AppendPayment(
		resp.GetUserId(), resp.GetAmountMinor(), []string{"voucher:" + id}, "", req.OtherTrxCode)
	if lerr != nil {
		log.Printf("gateway: ledger append failed for voucher %s: %v", id, lerr)
	} else {
		ledgerHash = base64.StdEncoding.EncodeToString(entry.Hash)
	}

	out := voucherJSON(resp)
	out["redeemed"] = true
	out["ledger_hash"] = ledgerHash
	writeJSON(w, http.StatusOK, out)
}

// voucherExpire flips past-due vouchers to expired and refunds the reserved credit.
func (a *API) voucherExpire(w http.ResponseWriter, r *http.Request) {
	resp, err := a.wallet.ExpireVouchers(r.Context(), &walletv1.ExpireVouchersRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"expired_count":  resp.GetExpiredCount(),
		"refunded_minor": resp.GetRefundedMinor(),
	})
}

// voucherPubkey exposes the wallet's voucher-signing key so a voucher can be verified offline.
func (a *API) voucherPubkey(w http.ResponseWriter, r *http.Request) {
	resp, err := a.wallet.GetVoucherPubKey(r.Context(), &walletv1.GetVoucherPubKeyRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"voucher_pubkey": base64.StdEncoding.EncodeToString(resp.GetVoucherPubkey()),
	})
}

// accountVouchers lists a user's vouchers for the app.
func (a *API) accountVouchers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := a.wallet.ListVouchers(r.Context(), &walletv1.ListVouchersRequest{UserId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(resp.GetVouchers()))
	for _, v := range resp.GetVouchers() {
		out = append(out, voucherJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"vouchers": out})
}
