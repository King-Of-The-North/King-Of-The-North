// Package service implements the WalletService gRPC surface, mapping proto requests
// onto the Day-Zero calc, the Postgres ledger (store), and the Moka client. It holds
// no money logic of its own — calc is the math, store is the authoritative ledger.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/calc"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/moka"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Wallet implements walletv1.WalletServiceServer.
type Wallet struct {
	walletv1.UnimplementedWalletServiceServer
	store *store.Store
	moka  moka.Client
}

// New builds the service over a ledger store and a Moka client (Mock for the demo).
func New(s *store.Store, m moka.Client) *Wallet {
	return &Wallet{store: s, moka: m}
}

// compile-time check: Wallet implements the generated server interface.
var _ walletv1.WalletServiceServer = (*Wallet)(nil)

// CalculateLimit computes the Day-Zero limit from a deposit and persists the account
// (deposit → L0 granted in one call — design decision). Re-deposit overwrites.
func (w *Wallet) CalculateLimit(ctx context.Context, req *walletv1.CalculateLimitRequest) (*walletv1.CalculateLimitResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetDepositMinor() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "deposit_minor must be positive")
	}

	limit, yield := calc.DayZero(
		req.GetDepositMinor(), req.GetApy(),
		req.GetCompoundingPerYear(), req.GetLockupYears(), req.GetRiskMargin())

	lockupEnd := time.Now().UTC().AddDate(int(req.GetLockupYears()), 0, 0)

	acct := store.Account{
		UserID:         req.GetUserId(),
		PrincipalMinor: req.GetDepositMinor(),
		ProjectedYield: yield,
		CreditLimit:    limit,
		LockupEndDate:  lockupEnd,
		PoolType:       "fixed", // only value for the demo (ADR-0001)
	}
	if err := w.store.Deposit(ctx, acct); err != nil {
		return nil, status.Errorf(codes.Internal, "persist deposit: %v", err)
	}

	return &walletv1.CalculateLimitResponse{
		CreditLimitMinor:    limit,
		ProjectedYieldMinor: yield,
		LockupEndDate:       lockupEnd.Format(time.RFC3339),
	}, nil
}

// GetAccount reads current account state for the app dashboard.
func (w *Wallet) GetAccount(ctx context.Context, req *walletv1.GetAccountRequest) (*walletv1.Account, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	a, err := w.store.GetAccount(ctx, req.GetUserId())
	if errors.Is(err, store.ErrAccountNotFound) {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get account: %v", err)
	}
	return toProtoAccount(a), nil
}

// ValidateTransaction sums the cart, atomically deducts it against available credit,
// then settles via Moka. The ledger deduction is authoritative; Moka is the external
// settle rubber-stamp (the Mock always succeeds for the demo).
func (w *Wallet) ValidateTransaction(ctx context.Context, req *walletv1.ValidateTransactionRequest) (*walletv1.ValidateTransactionResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetOtherTrxCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "other_trx_code required")
	}

	total, err := cartTotal(req.GetItems())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	res, err := w.store.Deduct(ctx, req.GetUserId(), total, req.GetOtherTrxCode())
	if errors.Is(err, store.ErrInsufficientCredit) {
		return &walletv1.ValidateTransactionResponse{
			Approved:      false,
			DeclineReason: "insufficient credit",
		}, nil
	}
	if errors.Is(err, store.ErrAccountNotFound) {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deduct: %v", err)
	}

	// Settle outside the ledger transaction — external call, no row lock held.
	settle, err := w.moka.Settle(ctx, moka.SettleRequest{
		OtherTrxCode: req.GetOtherTrxCode(),
		AmountMinor:  total,
	})
	if err != nil {
		// Ledger is already deducted; surface the error. A real failure path would
		// reverse the deduction (out of scope for the demo — Mock always succeeds).
		return nil, status.Errorf(codes.Internal, "settle: %v", err)
	}
	if err := w.store.SetMokaPaymentID(ctx, res.TrxID, settle.PaymentID); err != nil {
		return nil, status.Errorf(codes.Internal, "record settlement: %v", err)
	}

	return &walletv1.ValidateTransactionResponse{
		Approved:             true,
		RemainingCreditMinor: res.RemainingMinor,
		MokaPaymentId:        settle.PaymentID,
	}, nil
}

// nodeRewardProof is the decoded contribution_proof. The Gateway's metering layer
// decides the reward amount (DePIN economics live in the Gateway, ADR-0008); the
// Wallet just credits it. ref is a unique reference used as the audit trx code.
type nodeRewardProof struct {
	Minor int64  `json:"minor"`
	Ref   string `json:"ref"`
}

// CreditNodeReward converts a signed contribution proof into wallet credit. The
// reward is a normal ledger entry in the authoritative Postgres store — never minted
// on the phone (ADR-0008).
func (w *Wallet) CreditNodeReward(ctx context.Context, req *walletv1.CreditNodeRewardRequest) (*walletv1.CreditNodeRewardResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	var proof nodeRewardProof
	if err := json.Unmarshal(req.GetContributionProof(), &proof); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid contribution_proof")
	}
	if proof.Minor <= 0 {
		return nil, status.Error(codes.InvalidArgument, "reward must be positive")
	}
	if proof.Ref == "" {
		return nil, status.Error(codes.InvalidArgument, "contribution_proof ref required")
	}

	a, err := w.store.CreditNodeReward(ctx, req.GetUserId(), proof.Minor, proof.Ref)
	if errors.Is(err, store.ErrAccountNotFound) {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "credit reward: %v", err)
	}

	return &walletv1.CreditNodeRewardResponse{
		CreditedMinor:        proof.Minor,
		AvailableCreditMinor: a.AvailableCredit,
	}, nil
}

// Transfer atomically moves spendable credit from one user to another. Only
// available_credit moves (the locked principal stays put); the store enforces the
// no-negative-balance and no-deadlock invariants. This is an internal ledger move —
// no Moka settle (nothing leaves the custody rails).
func (w *Wallet) Transfer(ctx context.Context, req *walletv1.TransferRequest) (*walletv1.TransferResponse, error) {
	if req.GetFromUserId() == "" || req.GetToUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "from_user_id and to_user_id required")
	}
	if req.GetFromUserId() == req.GetToUserId() {
		return nil, status.Error(codes.InvalidArgument, "cannot transfer to self")
	}
	if req.GetAmountMinor() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_minor must be positive")
	}
	if req.GetRef() == "" {
		return nil, status.Error(codes.InvalidArgument, "ref required")
	}

	res, err := w.store.TransferCredit(ctx, req.GetFromUserId(), req.GetToUserId(), req.GetAmountMinor(), req.GetRef())
	if errors.Is(err, store.ErrSelfTransfer) {
		return nil, status.Error(codes.InvalidArgument, "cannot transfer to self")
	}
	if errors.Is(err, store.ErrAccountNotFound) {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if errors.Is(err, store.ErrInsufficientCredit) {
		return nil, status.Error(codes.FailedPrecondition, "insufficient credit")
	}
	if errors.Is(err, store.ErrDuplicateRef) {
		return nil, status.Error(codes.AlreadyExists, "transfer ref already used")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "transfer: %v", err)
	}

	return &walletv1.TransferResponse{
		FromRemainingMinor: res.FromRemainingMinor,
		ToAvailableMinor:   res.ToAvailableMinor,
		FromTrxCode:        res.FromTrxCode,
		ToTrxCode:          res.ToTrxCode,
	}, nil
}

// cartTotal sums line items in minor units, guarding against empty carts and overflow.
func cartTotal(items []*walletv1.CartItem) (int64, error) {
	if len(items) == 0 {
		return 0, errors.New("cart is empty")
	}
	var total int64
	for _, it := range items {
		if it.GetPriceMinor() < 0 {
			return 0, errors.New("negative price")
		}
		if it.GetQuantity() == 0 {
			return 0, errors.New("zero quantity")
		}
		line := it.GetPriceMinor() * int64(it.GetQuantity())
		// overflow guard: line must not be smaller than a factor when both positive
		if it.GetPriceMinor() != 0 && line/int64(it.GetQuantity()) != it.GetPriceMinor() {
			return 0, errors.New("line total overflow")
		}
		newTotal := total + line
		if newTotal < total {
			return 0, errors.New("cart total overflow")
		}
		total = newTotal
	}
	if total <= 0 {
		return 0, errors.New("cart total must be positive")
	}
	return total, nil
}

func toProtoAccount(a store.Account) *walletv1.Account {
	return &walletv1.Account{
		UserId:               a.UserID,
		PrincipalMinor:       a.PrincipalMinor,
		ProjectedYieldMinor:  a.ProjectedYield,
		CreditLimitMinor:     a.CreditLimit,
		AvailableCreditMinor: a.AvailableCredit,
		LtvRatio:             a.LTVRatio,
		LockupEndDate:        a.LockupEndDate.Format(time.RFC3339),
		PoolType:             a.PoolType,
	}
}
