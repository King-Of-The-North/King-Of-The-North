package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"google.golang.org/grpc"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/ledgerp2p"
	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
)

// fakeWallet is a minimal in-test WalletServiceClient that serves the device registry
// from memory. The embedded nil interface supplies the many RPCs these tests don't call.
type fakeWallet struct {
	walletv1.WalletServiceClient
	mu      sync.Mutex
	devices map[string][][]byte // user_id -> active pubkeys
}

func newFakeWallet() *fakeWallet { return &fakeWallet{devices: map[string][][]byte{}} }

func (f *fakeWallet) enroll(userID string, pub []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.devices[userID] = append(f.devices[userID], pub)
}

func (f *fakeWallet) EnrollDevice(_ context.Context, in *walletv1.EnrollDeviceRequest, _ ...grpc.CallOption) (*walletv1.DeviceList, error) {
	f.enroll(in.GetUserId(), in.GetDevicePubkey())
	return f.ListDevices(context.Background(), &walletv1.ListDevicesRequest{UserId: in.GetUserId()})
}

func (f *fakeWallet) ListDevices(_ context.Context, in *walletv1.ListDevicesRequest, _ ...grpc.CallOption) (*walletv1.DeviceList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := &walletv1.DeviceList{UserId: in.GetUserId()}
	for _, p := range f.devices[in.GetUserId()] {
		out.Devices = append(out.Devices, &walletv1.Device{DevicePubkey: p, Active: true})
	}
	return out, nil
}

// ValidateTransaction always approves — the signed-pay tests care about the signature
// gate in the gateway, not the wallet's deduct logic (covered by the wallet's own tests).
func (f *fakeWallet) ValidateTransaction(_ context.Context, _ *walletv1.ValidateTransactionRequest, _ ...grpc.CallOption) (*walletv1.ValidateTransactionResponse, error) {
	return &walletv1.ValidateTransactionResponse{Approved: true, RemainingCreditMinor: 999, MokaPaymentId: "mock"}, nil
}

// TestLedgerWSHandshakeReplicateAck drives the whole phone-node path over a real
// WebSocket: signed device handshake -> welcome -> back-fill -> live entry -> ACK ->
// metered reward. The ledger/devices stores are real; the wallet client is unused here.
func TestLedgerWSHandshakeReplicateAck(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	cluster := ledgerp2p.NewCluster(priv, 0)
	fake := newFakeWallet()
	api := New(fake, cluster, nil, nil, DepinConfig{RewardPerEntryMinor: 5, CloudCostPerEntryMinor: 12})
	mux := http.NewServeMux()
	api.Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	const userID = "user-1"
	dpub, dpriv, _ := ed25519.GenerateKey(rand.Reader)
	fake.enroll(userID, dpub)
	// One entry exists before the phone connects → it must arrive as back-fill.
	if _, err := cluster.AppendPayment(userID, 1000, []string{"pre"}, "", "pre-1"); err != nil {
		t.Fatalf("pre append: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/ledger/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// 1. challenge → sign the nonce with the device key.
	var chal struct {
		Type  string `json:"type"`
		Nonce string `json:"nonce"`
	}
	if err := wsjson.Read(ctx, conn, &chal); err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	if chal.Type != "challenge" {
		t.Fatalf("want challenge, got %q", chal.Type)
	}
	nonce, _ := base64.StdEncoding.DecodeString(chal.Nonce)
	if err := wsjson.Write(ctx, conn, map[string]any{
		"type":          "auth",
		"user_id":       userID,
		"device_pubkey": base64.StdEncoding.EncodeToString(dpub),
		"sig":           base64.StdEncoding.EncodeToString(ed25519.Sign(dpriv, nonce)),
	}); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	// 2. welcome (chain length 1 at connect).
	var wel struct {
		Type string `json:"type"`
		Len  int    `json:"len"`
	}
	if err := wsjson.Read(ctx, conn, &wel); err != nil {
		t.Fatalf("read welcome: %v", err)
	}
	if wel.Type != "welcome" || wel.Len != 1 {
		t.Fatalf("bad welcome: %+v", wel)
	}

	// 3. back-fill entry (seq 0) → ack it.
	if e := readEntry(t, ctx, conn); e.Content.Seq != 0 {
		t.Fatalf("want back-fill seq 0, got %d", e.Content.Seq)
	}
	ackEntry(t, ctx, conn, 0)

	// 4. a live payment appends → phone receives it live (seq 1) → ack.
	if _, err := cluster.AppendPayment(userID, 2000, []string{"live"}, "", "live-1"); err != nil {
		t.Fatalf("live append: %v", err)
	}
	if e := readEntry(t, ctx, conn); e.Content.Seq != 1 {
		t.Fatalf("want live seq 1, got %d", e.Content.Seq)
	}
	ackEntry(t, ctx, conn, 1)

	// 5. metering: back-fill counts lifetime only; the live ack is the rewardable one.
	waitFor(t, 2*time.Second, func() bool {
		for _, m := range cluster.Meter() {
			if m.Owner == userID {
				return m.Pending == 1 && m.Lifetime == 2
			}
		}
		return false
	}, "meter never reached pending=1 lifetime=2")

	// The phone shows up as a REMOTE node in the cluster view.
	var remote bool
	for _, n := range cluster.Nodes() {
		if n.Owner == userID && n.Remote {
			remote = true
		}
	}
	if !remote {
		t.Fatal("phone not reported as a remote node")
	}
}

// TestLedgerWSRejectsUnenrolledDevice: a phone whose key isn't enrolled is refused.
func TestLedgerWSRejectsUnenrolledDevice(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	cluster := ledgerp2p.NewCluster(priv, 0)
	api := New(newFakeWallet(), cluster, nil, nil, DepinConfig{})
	mux := http.NewServeMux()
	api.Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/ledger/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	var chal struct {
		Type  string `json:"type"`
		Nonce string `json:"nonce"`
	}
	if err := wsjson.Read(ctx, conn, &chal); err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	// Valid signature, but the device was never enrolled → server must reject.
	dpub, dpriv, _ := ed25519.GenerateKey(rand.Reader)
	nonce, _ := base64.StdEncoding.DecodeString(chal.Nonce)
	_ = wsjson.Write(ctx, conn, map[string]any{
		"type":          "auth",
		"user_id":       "nobody",
		"device_pubkey": base64.StdEncoding.EncodeToString(dpub),
		"sig":           base64.StdEncoding.EncodeToString(ed25519.Sign(dpriv, nonce)),
	})
	// The next read must fail (connection closed with a policy violation).
	var msg map[string]any
	if err := wsjson.Read(ctx, conn, &msg); err == nil {
		t.Fatal("expected connection close after failed auth, got a message")
	}
}

// --- helpers ---

func readEntry(t *testing.T, ctx context.Context, conn *websocket.Conn) ledgerp2p.Entry {
	t.Helper()
	var frame struct {
		Type  string          `json:"type"`
		Entry ledgerp2p.Entry `json:"entry"`
	}
	if err := wsjson.Read(ctx, conn, &frame); err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if frame.Type != "entry" {
		t.Fatalf("want entry frame, got %q", frame.Type)
	}
	return frame.Entry
}

func ackEntry(t *testing.T, ctx context.Context, conn *websocket.Conn, seq int) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "ack", "seq": seq}); err != nil {
		t.Fatalf("write ack: %v", err)
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}
