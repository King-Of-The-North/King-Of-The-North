package service

import (
	"context"
	"crypto/ed25519"
	"errors"

	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
	"github.com/king-of-the-north/king-of-the-north/wallet/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnrollDevice binds a phone's Ed25519 public key to a user (P2P WebSocket + signed-pay
// auth). Only the public key is stored; the private key never leaves the phone.
func (w *Wallet) EnrollDevice(ctx context.Context, req *walletv1.EnrollDeviceRequest) (*walletv1.DeviceList, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if len(req.GetDevicePubkey()) != ed25519.PublicKeySize {
		return nil, status.Error(codes.InvalidArgument, "device_pubkey must be a 32-byte Ed25519 key")
	}
	if err := w.store.EnrollDevice(ctx, req.GetUserId(), req.GetDevicePubkey()); err != nil {
		return nil, mapDeviceErr(err)
	}
	return w.listDevices(ctx, req.GetUserId())
}

// ListDevices returns a user's active device public keys.
func (w *Wallet) ListDevices(ctx context.Context, req *walletv1.ListDevicesRequest) (*walletv1.DeviceList, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	return w.listDevices(ctx, req.GetUserId())
}

// RevokeDevice marks a single device key revoked.
func (w *Wallet) RevokeDevice(ctx context.Context, req *walletv1.RevokeDeviceRequest) (*walletv1.DeviceList, error) {
	if req.GetUserId() == "" || len(req.GetDevicePubkey()) != ed25519.PublicKeySize {
		return nil, status.Error(codes.InvalidArgument, "user_id and 32-byte device_pubkey required")
	}
	if err := w.store.RevokeDevice(ctx, req.GetUserId(), req.GetDevicePubkey()); err != nil {
		return nil, mapDeviceErr(err)
	}
	return w.listDevices(ctx, req.GetUserId())
}

// RebindDevices is account recovery (ADR-0011): revoke all existing device keys and
// enroll a new one, so a lost/new phone takes over. The balance (anchored to user_id) is
// untouched. NOTE: recovery is self-asserted here — a production build gates it behind an
// identity proof (KYC/OTP), which is out of scope for the demo.
func (w *Wallet) RebindDevices(ctx context.Context, req *walletv1.RebindDevicesRequest) (*walletv1.DeviceList, error) {
	if req.GetUserId() == "" || len(req.GetNewDevicePubkey()) != ed25519.PublicKeySize {
		return nil, status.Error(codes.InvalidArgument, "user_id and 32-byte new_device_pubkey required")
	}
	if err := w.store.RebindDevices(ctx, req.GetUserId(), req.GetNewDevicePubkey()); err != nil {
		return nil, mapDeviceErr(err)
	}
	return w.listDevices(ctx, req.GetUserId())
}

// listDevices reads the active set and maps it to the proto DeviceList.
func (w *Wallet) listDevices(ctx context.Context, userID string) (*walletv1.DeviceList, error) {
	devices, err := w.store.ListActiveDevices(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list devices: %v", err)
	}
	out := make([]*walletv1.Device, 0, len(devices))
	for _, d := range devices {
		out = append(out, &walletv1.Device{
			DevicePubkey: d.PubKey,
			Active:       d.Active,
			CreatedAt:    d.CreatedAt,
		})
	}
	return &walletv1.DeviceList{UserId: userID, Devices: out}, nil
}

func mapDeviceErr(err error) error {
	if errors.Is(err, store.ErrAccountNotFound) {
		return status.Error(codes.InvalidArgument, "invalid user_id")
	}
	return status.Errorf(codes.Internal, "device op: %v", err)
}
