package httpapi

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/king-of-the-north/king-of-the-north/gateway/internal/ledgerp2p"
)

func postJSON(t *testing.T, url string, body any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func TestPaySignedAcceptsAndRejects(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	cluster := ledgerp2p.NewCluster(priv, 0)
	fake := newFakeWallet()
	api := New(fake, cluster, nil, nil, DepinConfig{})
	mux := http.NewServeMux()
	api.Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	url := srv.URL + "/v1/pay/signed"

	const user = "u1"
	dpub, dpriv, _ := ed25519.GenerateKey(rand.Reader)
	fake.enroll(user, dpub)

	items := []map[string]any{{"sku": "S", "name": "Coffee", "price_minor": 4500, "quantity": 2}}
	// Must reproduce canonicalCart exactly: "<user>|<trx>|<total>|<sku>:<qty>".
	canonical := []byte(fmt.Sprintf("%s|%s|%d|%s", user, "trx1", 9000, "S:2"))

	// 1. valid signature by the enrolled device → 200.
	good := map[string]any{
		"user_id": user, "other_trx_code": "trx1", "items": items,
		"device_pubkey": base64.StdEncoding.EncodeToString(dpub),
		"sig":           base64.StdEncoding.EncodeToString(ed25519.Sign(dpriv, canonical)),
	}
	if code := postJSON(t, url, good); code != http.StatusOK {
		t.Fatalf("valid signed pay: want 200, got %d", code)
	}

	// 2. a key that isn't enrolled → 401 (rejected at the enrollment check).
	opub, opriv, _ := ed25519.GenerateKey(rand.Reader)
	canon2 := []byte(fmt.Sprintf("%s|%s|%d|%s", user, "trx2", 9000, "S:2"))
	unenrolled := map[string]any{
		"user_id": user, "other_trx_code": "trx2", "items": items,
		"device_pubkey": base64.StdEncoding.EncodeToString(opub),
		"sig":           base64.StdEncoding.EncodeToString(ed25519.Sign(opriv, canon2)),
	}
	if code := postJSON(t, url, unenrolled); code != http.StatusUnauthorized {
		t.Fatalf("unenrolled device: want 401, got %d", code)
	}

	// 3. enrolled device but signature over a different cart (tampered) → 401.
	tampered := map[string]any{
		"user_id": user, "other_trx_code": "trx3", "items": items,
		"device_pubkey": base64.StdEncoding.EncodeToString(dpub),
		"sig":           base64.StdEncoding.EncodeToString(ed25519.Sign(dpriv, []byte("wrong-message"))),
	}
	if code := postJSON(t, url, tampered); code != http.StatusUnauthorized {
		t.Fatalf("tampered signature: want 401, got %d", code)
	}
}
