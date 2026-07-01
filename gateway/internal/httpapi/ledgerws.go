package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/ledgerp2p"
	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"
)

// wsPingInterval is how often the server pings an idle phone socket to keep it alive
// and detect dead connections.
const wsPingInterval = 20 * time.Second

// ledgerWS turns a phone into a REAL P2P node (ADR-0008): it authenticates the phone by
// a signed device-key handshake, registers it as a remote replica, back-fills the chain,
// then streams every new signed entry live. The phone verifies each entry locally (it is
// sent the anchor's public key) and ACKs it; each ACK meters a rewardable contribution
// (proof of replication). Money never crosses the socket — only the signed audit log
// (ADR-0005). Losing the socket loses nothing: the anchor and other nodes keep the copy.
//
// Protocol (JSON frames):
//
//	S->C {"type":"challenge","nonce":"<b64>"}
//	C->S {"type":"auth","user_id":..,"device_pubkey":"<b64>","sig":"<b64 sign(nonce)>"}
//	S->C {"type":"welcome","node_id":N,"anchor_pubkey":"<b64>","len":L}
//	S->C {"type":"entry","entry":{..}}    (back-fill from ?since=, then live)
//	S->C {"type":"ping"}
//	C->S {"type":"ack","seq":S}           (stored+verified entry S -> meters reward)
//	C->S {"type":"pong"}
func (a *API) ledgerWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Demo: phones connect cross-origin without an Origin header; identity is proven
		// by the device-key handshake below, not by Origin.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return // Accept already wrote the error response
	}
	defer func() { _ = conn.CloseNow() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userID, ok := a.wsHandshake(ctx, conn)
	if !ok {
		_ = conn.Close(websocket.StatusPolicyViolation, "device authentication failed")
		return
	}

	// Register the phone as a remote replica and make sure it is deregistered on exit.
	id, backfill, ch := a.ledger.AddWSReplica(userID)
	defer a.ledger.KillReplica(id)

	if err := wsjson.Write(ctx, conn, map[string]any{
		"type":          "welcome",
		"node_id":       id,
		"anchor_pubkey": base64.StdEncoding.EncodeToString(a.ledger.PublicKey()),
		"len":           len(backfill),
	}); err != nil {
		return
	}

	// since lets a reconnecting phone skip entries it already holds.
	since := 0
	if v := r.URL.Query().Get("since"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			since = n
		}
	}

	// Single writer goroutine: back-fill, then live entries + keepalive pings. (coder
	// /websocket allows one concurrent reader and one writer; the read loop below is the
	// reader.) cancel() on any write error tears the whole connection down.
	go a.wsWriter(ctx, cancel, conn, backfill, since, ch)

	// Reader loop: acks (meter the reward) and pongs. Returns on disconnect.
	for {
		var msg struct {
			Type string `json:"type"`
			Seq  int    `json:"seq"`
		}
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return // client gone or context cancelled
		}
		switch msg.Type {
		case "ack":
			a.ledger.AckWSReplica(id, msg.Seq)
		case "pong":
			// keepalive acknowledged
		}
	}
}

// wsWriter streams the back-fill (from since) then live entries and periodic pings.
func (a *API) wsWriter(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, backfill []ledgerp2p.Entry, since int, ch chan ledgerp2p.Entry) {
	defer cancel()

	for _, e := range backfill {
		if int(e.Content.Seq) < since {
			continue // phone already has it
		}
		if err := wsjson.Write(ctx, conn, entryFrame(e)); err != nil {
			return
		}
	}

	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return // replica killed
			}
			if err := wsjson.Write(ctx, conn, entryFrame(e)); err != nil {
				return
			}
		case <-ticker.C:
			if err := wsjson.Write(ctx, conn, map[string]any{"type": "ping"}); err != nil {
				return
			}
		}
	}
}

// wsHandshake proves the connecting phone holds the private key for an enrolled device
// public key bound to the user it claims. Returns the authenticated user_id, or false.
func (a *API) wsHandshake(ctx context.Context, conn *websocket.Conn) (string, bool) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", false
	}
	if err := wsjson.Write(ctx, conn, map[string]any{
		"type":  "challenge",
		"nonce": base64.StdEncoding.EncodeToString(nonce),
	}); err != nil {
		return "", false
	}

	authCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var auth struct {
		Type         string `json:"type"`
		UserID       string `json:"user_id"`
		DevicePubKey string `json:"device_pubkey"`
		Sig          string `json:"sig"`
	}
	if err := wsjson.Read(authCtx, conn, &auth); err != nil {
		return "", false
	}
	if auth.Type != "auth" || auth.UserID == "" {
		return "", false
	}
	pub, err := base64.StdEncoding.DecodeString(auth.DevicePubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return "", false
	}
	sig, err := base64.StdEncoding.DecodeString(auth.Sig)
	if err != nil {
		return "", false
	}
	// Both must hold: the key is an active enrolled device for this user (looked up in
	// the wallet), AND the phone signed our nonce with the matching private key.
	if !a.deviceEnrolled(ctx, auth.UserID, pub) {
		log.Printf("gateway: ws handshake: device not enrolled for %s", auth.UserID)
		return "", false
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), nonce, sig) {
		log.Printf("gateway: ws handshake: bad signature for %s", auth.UserID)
		return "", false
	}
	return auth.UserID, true
}

// deviceEnrolled reports whether pub is an active device for userID, per the wallet
// registry. Any lookup error is treated as "not enrolled" (fail closed).
func (a *API) deviceEnrolled(ctx context.Context, userID string, pub []byte) bool {
	resp, err := a.wallet.ListDevices(ctx, &walletv1.ListDevicesRequest{UserId: userID})
	if err != nil {
		log.Printf("gateway: list devices for %s: %v", userID, err)
		return false
	}
	for _, d := range resp.GetDevices() {
		if bytes.Equal(d.GetDevicePubkey(), pub) {
			return true
		}
	}
	return false
}

// entryFrame wraps a ledger entry in the "entry" message envelope.
func entryFrame(e ledgerp2p.Entry) map[string]any {
	return map[string]any{"type": "entry", "entry": e}
}
