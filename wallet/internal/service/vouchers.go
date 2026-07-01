package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// defaultVoucherTTL bounds how long an offline voucher is spendable (ADR-0012).
const defaultVoucherTTL = time.Hour

// voucherMessage is the canonical byte string the wallet signs, so a voucher can be
// verified offline against the wallet's public key (GetVoucherPubKey). Deterministic and
// reproducible: "<id>|<user_id>|<amount>|<b64 nonce>|<expires_unix>".
func voucherMessage(id, userID string, amountMinor int64, nonce []byte, expiresAt time.Time) []byte {
	return []byte(fmt.Sprintf("%s|%s|%d|%s|%d",
		id, userID, amountMinor, base64.StdEncoding.EncodeToString(nonce), expiresAt.Unix()))
}

// IssueVoucher reserves amount from available_credit and returns a wallet-signed voucher
// the phone can spend offline up to the cap (ADR-0012).
func (w *Wallet) IssueVoucher(ctx context.Context, req *walletv1.IssueVoucherRequest) (*walletv1.Voucher, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetAmountMinor() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_minor must be positive")
	}
	ttl := defaultVoucherTTL
	if req.GetTtlSeconds() > 0 {
		ttl = time.Duration(req.GetTtlSeconds()) * time.Second
	}

	id := uuid.NewString()
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, status.Errorf(codes.Internal, "nonce: %v", err)
	}
	expiresAt := time.Now().UTC().Add(ttl)
	sig := ed25519.Sign(w.voucherPriv, voucherMessage(id, req.GetUserId(), req.GetAmountMinor(), nonce, expiresAt))

	err := w.store.IssueVoucher(ctx, id, req.GetUserId(), req.GetAmountMinor(), nonce, sig, expiresAt)
	if errors.Is(err, store.ErrAccountNotFound) {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if errors.Is(err, store.ErrInsufficientCredit) {
		return nil, status.Error(codes.FailedPrecondition, "insufficient credit")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "issue voucher: %v", err)
	}

	return &walletv1.Voucher{
		Id:          id,
		UserId:      req.GetUserId(),
		AmountMinor: req.GetAmountMinor(),
		Status:      "issued",
		Nonce:       nonce,
		Sig:         sig,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		IssuedAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// RedeemVoucher finalizes a voucher exactly once. The credit was reserved at issue, so
// this only records the settlement; a second redeem is rejected (bounded double-spend).
func (w *Wallet) RedeemVoucher(ctx context.Context, req *walletv1.RedeemVoucherRequest) (*walletv1.Voucher, error) {
	if req.GetVoucherId() == "" || req.GetOtherTrxCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "voucher_id and other_trx_code required")
	}
	v, err := w.store.RedeemVoucher(ctx, req.GetVoucherId(), req.GetOtherTrxCode())
	if errors.Is(err, store.ErrVoucherNotFound) {
		return nil, status.Error(codes.NotFound, "voucher not found")
	}
	if errors.Is(err, store.ErrAlreadyRedeemed) {
		return nil, status.Error(codes.AlreadyExists, "voucher already redeemed")
	}
	if errors.Is(err, store.ErrVoucherExpired) {
		return nil, status.Error(codes.FailedPrecondition, "voucher expired")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "redeem voucher: %v", err)
	}
	return &walletv1.Voucher{
		Id:            v.ID,
		UserId:        v.UserID,
		AmountMinor:   v.AmountMinor,
		Status:        v.Status,
		RedeemTrxCode: v.RedeemTrxCode,
	}, nil
}

// ExpireVouchers flips past-due vouchers to expired and refunds the reserved credit.
func (w *Wallet) ExpireVouchers(ctx context.Context, _ *walletv1.ExpireVouchersRequest) (*walletv1.ExpireVouchersResponse, error) {
	count, refunded, err := w.store.ExpireVouchers(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "expire vouchers: %v", err)
	}
	return &walletv1.ExpireVouchersResponse{ExpiredCount: int32(count), RefundedMinor: refunded}, nil
}

// ListVouchers returns a user's vouchers for the app.
func (w *Wallet) ListVouchers(ctx context.Context, req *walletv1.ListVouchersRequest) (*walletv1.VoucherList, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	vs, err := w.store.ListVouchers(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list vouchers: %v", err)
	}
	out := make([]*walletv1.Voucher, 0, len(vs))
	for _, v := range vs {
		out = append(out, &walletv1.Voucher{
			Id:            v.ID,
			UserId:        v.UserID,
			AmountMinor:   v.AmountMinor,
			Status:        v.Status,
			Nonce:         v.Nonce,
			Sig:           v.Sig,
			ExpiresAt:     v.ExpiresAt.Format(time.RFC3339),
			IssuedAt:      v.IssuedAt.Format(time.RFC3339),
			RedeemTrxCode: v.RedeemTrxCode,
		})
	}
	return &walletv1.VoucherList{Vouchers: out}, nil
}

// GetVoucherPubKey returns the wallet's voucher-signing public key so a merchant can
// verify a voucher offline.
func (w *Wallet) GetVoucherPubKey(_ context.Context, _ *walletv1.GetVoucherPubKeyRequest) (*walletv1.GetVoucherPubKeyResponse, error) {
	return &walletv1.GetVoucherPubKeyResponse{VoucherPubkey: w.voucherPub}, nil
}
