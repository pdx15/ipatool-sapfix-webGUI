package gsa

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/majd/ipatool/v2/pkg/anisette"
	"howett.net/plist"
)

const (
	testEmail    = "user@example.com"
	testPassword = "correct horse battery staple"
	testSalt     = "beb25379d1a8581eb5a727673a2441ee"
	// Server private value b (from the independent srptools sha256/2048 vector).
	testServerB = "e487cb59d31ac550471e81f00f6928e01dda08e974a004f49e61f5d105284d20"
)

// gsaTestServer implements the SRP server side of the GSA handshake so the
// client can be exercised end-to-end over a real HTTP round trip.
type gsaTestServer struct {
	salt     []byte
	b        []byte
	x        []byte
	B        []byte
	k        []byte
	nPadded  []byte
	gPad     []byte
	force409 bool

	mu       sync.Mutex
	clientA  []byte
	sessionK []byte
}

func newGSATestServer(force409 bool) *gsaTestServer {
	N := GroupN()
	G := GroupG()

	s := &gsaTestServer{
		salt:     mustHexDecode(testSalt),
		b:        mustHexDecode(testServerB),
		x:        AppleS2K(testPassword, mustHexDecode(testSalt), 20000, false),
		nPadded:  padTo(N, GroupLength),
		gPad:     gPadded(GroupLength),
		force409: force409,
	}
	s.k = computeK(s.nPadded, s.gPad)

	// B = (k*v + g^b) mod N
	v := new(big.Int).Exp(G, new(big.Int).SetBytes(s.x), N)
	gb := new(big.Int).Exp(G, new(big.Int).SetBytes(s.b), N)
	kv := new(big.Int).Mul(new(big.Int).SetBytes(s.k), v)
	kv.Mod(kv, N)
	B := new(big.Int).Add(kv, gb)
	B.Mod(B, N)
	s.B = padTo(B, GroupLength)

	return s
}

func (s *gsaTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if r.Body != nil {
		data, _ := io.ReadAll(r.Body)
		_, _ = plist.Unmarshal(data, &body)
	}

	req, _ := body["Request"].(map[string]interface{})

	switch coerceStr(req["o"]) {
	case "init":
		s.handleInit(w, req)
	case "complete":
		s.handleComplete(w, req)
	default:
		http.Error(w, "unknown op", http.StatusBadRequest)
	}
}

func (s *gsaTestServer) handleInit(w http.ResponseWriter, req map[string]interface{}) {
	aBytes, _ := dataVal(req, "A2k")

	s.mu.Lock()
	s.clientA = append([]byte(nil), aBytes...)
	s.mu.Unlock()

	respondPlist(w, map[string]interface{}{
		"Response": map[string]interface{}{
			"s":  s.salt,
			"B":  s.B,
			"c":  "test-challenge",
			"i":  20000,
			"sp": "s2k",
		},
	})
}

func (s *gsaTestServer) handleComplete(w http.ResponseWriter, req map[string]interface{}) {
	m1Bytes, _ := dataVal(req, "M1")

	s.mu.Lock()
	A := new(big.Int).SetBytes(s.clientA)
	Apadded := padTo(A, GroupLength)
	Bpadded := s.B
	u := computeU(Apadded, Bpadded)

	N := GroupN()
	G := GroupG()
	v := new(big.Int).Exp(G, new(big.Int).SetBytes(s.x), N)
	vu := new(big.Int).Exp(v, new(big.Int).SetBytes(u), N)
	Au := new(big.Int).Mul(A, vu)
	Au.Mod(Au, N)
	S := new(big.Int).Exp(Au, new(big.Int).SetBytes(s.b), N)
	Spadded := padTo(S, GroupLength)
	K := sessionKey(Spadded)
	s.sessionK = K

	wantM1 := computeM1(s.nPadded, s.gPad, []byte(testEmail), s.salt, Apadded, Bpadded, K)
	s.mu.Unlock()

	if !bytes.Equal(m1Bytes, wantM1) {
		http.Error(w, "M1 mismatch", http.StatusForbidden)
		return
	}

	var spd map[string]interface{}
	if s.force409 {
		spd = map[string]interface{}{
			"status-code": 409,
			"sm":          "Verify your identity",
			"adsid":       "1234567890",
			"GsIdmsToken": "gs-idms-token-value",
		}
	} else {
		spd = map[string]interface{}{
			"DsPrsId":     "1234567890",
			"adsid":       "1234567890",
			"GsIdmsToken": "gs-idms-token-value",
			"fn":          "Alice",
			"ln":          "Smith",
			"t": map[string]interface{}{
				"com.apple.gs.idms.pet": map[string]interface{}{"token": "pet-token-value"},
				"com.apple.gs.idms.hb":  map[string]interface{}{"token": "hb-token-value"},
			},
		}
	}

	spdBytes, err := plist.Marshal(spd, plist.BinaryFormat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	key, iv := SPDKeyIV(K)
	encrypted := encryptForTest(key, iv, spdBytes)

	respondPlist(w, map[string]interface{}{
		"Response": map[string]interface{}{
			"Result": "Allow",
			"spd":    encrypted,
		},
	})
}

// respondPlist writes an XML plist response.
func respondPlist(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/x-xml-plist")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func encryptForTest(key, iv, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	padLen := block.BlockSize() - len(plaintext)%block.BlockSize()
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out
}

func newTestClient(serverURL string) *Client {
	client := NewClient(nil)
	client.GSAURL = serverURL
	client.ValidateURL = serverURL + "/validate"
	return client
}

// TestClientLoginEndToEnd drives a complete SRP-6a handshake against an
// in-process SRP server and asserts the account tokens extracted from the
// decrypted spd payload.
func TestClientLoginEndToEnd(t *testing.T) {
	server := newGSATestServer(false)
	srv := httptest.NewServer(server)
	defer srv.Close()

	client := newTestClient(srv.URL)

	acc, err := client.Login(testEmail, testPassword, anisette.Data{
		OTP:        "otp",
		MachineID:  "machine-id",
		Locale:     "en_US",
		Timezone:   "PST",
		ClientTime: "2026-01-01T00:00:00Z",
	}, "")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if acc.Email != testEmail {
		t.Fatalf("Email = %q, want %q", acc.Email, testEmail)
	}

	if acc.DirectoryServicesID != "1234567890" {
		t.Fatalf("DirectoryServicesID = %q", acc.DirectoryServicesID)
	}

	if acc.GsIDMSToken != "gs-idms-token-value" {
		t.Fatalf("GsIDMSToken = %q", acc.GsIDMSToken)
	}

	if acc.PETToken != "pet-token-value" {
		t.Fatalf("PETToken = %q", acc.PETToken)
	}

	if acc.HBToken != "hb-token-value" {
		t.Fatalf("HBToken = %q", acc.HBToken)
	}

	if acc.Name != "Alice Smith" {
		t.Fatalf("Name = %q", acc.Name)
	}
}

// TestClientLoginRequiresAuthCode asserts that a status-code 409 inside spd
// surfaces ErrAuthCodeRequired when no code is supplied.
func TestClientLoginRequiresAuthCode(t *testing.T) {
	server := newGSATestServer(true)
	srv := httptest.NewServer(server)
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.Login(testEmail, testPassword, anisette.Data{OTP: "o", MachineID: "m"}, "")
	if !errors.Is(err, ErrAuthCodeRequired) {
		t.Fatalf("expected ErrAuthCodeRequired, got %v", err)
	}
}

// TestClientLoginInvalidAuthCode asserts that a rejected 2FA code surfaces
// ErrInvalidAuthCode.
func TestClientLoginInvalidAuthCode(t *testing.T) {
	server := newGSATestServer(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/validate" {
			// Apple returns HTTP 200 even on error; report ec != 0 in the body.
			respondPlist(w, map[string]interface{}{"ec": -1, "em": "invalid code"})
			return
		}

		server.ServeHTTP(w, r)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)

	_, err := client.Login(testEmail, testPassword, anisette.Data{OTP: "o", MachineID: "m"}, "000000")
	if !errors.Is(err, ErrInvalidAuthCode) {
		t.Fatalf("expected ErrInvalidAuthCode, got %v", err)
	}
}
