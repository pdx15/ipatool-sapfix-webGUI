// Package gsa implements Apple's GrandSlam Authentication (GSA) client
// handshake: SRP-6a with the RFC 5054 2048-bit group followed by the "s2k"
// password derivation Apple uses for Apple ID authentication.
//
// The SRP math in this file is a pure-Go port of the reference implementation
// used by the native ipatool engine (gsa.cpp / srp.cpp). It deliberately keeps
// every protocol value as fixed-width big-endian bytes, because Apple's k, u,
// M1 and session-key hashes all operate on fixed-width values.
package gsa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// GroupLength is the byte length of the RFC 5054 2048-bit modulus N.
const GroupLength = 256

// n2048 is the SRP modulus from RFC 5054 Appendix A (the NG_2048 group that
// Apple's GSA endpoint uses — NOT the RFC 3526 IKE prime of the same size).
var n2048 = new(big.Int).SetBytes(mustHexDecode(
	"AC6BDB41324A9A9BF166DE5E1389582F" +
		"AF72B6651987EE07FC3192943DB56050" +
		"A37329CBB4A099ED8193E0757767A13D" +
		"D52312AB4B03310DCD7F48A9DA04FD50" +
		"E8083969EDB767B0CF6095179A163AB3" +
		"661A05FBD5FAAAE82918A9962F0B93B8" +
		"55F97993EC975EEAA80D740ADBF4FF74" +
		"7359D041D5C33EA71D281E446B14773B" +
		"CA97B43A23FB801676BD207A436C6481" +
		"F1D2B9078717461A5B9D32E688F87748" +
		"544523B524B0D57D5EA77A2775D2ECFA" +
		"032CFBDBF52FB3786160279004E57AE6" +
		"AF874E7303CE53299CCC041C7BC308D8" +
		"2A5698F3A8D0C38271AE35F8E9DBFBB6" +
		"94B5C803D89F7AE435DE236D525F5475" +
		"9B65E372FCD68EF20FA7111F9E4AFF73"))

// g2048 is the SRP generator (g = 2).
var g2048 = big.NewInt(2)

// GroupN returns a copy of the RFC 5054 2048-bit modulus N.
func GroupN() *big.Int { return new(big.Int).Set(n2048) }

// GroupG returns a copy of the generator g = 2.
func GroupG() *big.Int { return new(big.Int).Set(g2048) }

// gPadded returns g left-padded with zeros to the given byte length. Apple's
// k/u/M1 hashes operate on fixed-width values, so g is always padded to the
// modulus length even though the integer itself is just 2.
func gPadded(length int) []byte {
	out := make([]byte, length)
	out[length-1] = 0x02
	return out
}

// padTo serializes n as big-endian bytes zero-padded on the left to length.
func padTo(n *big.Int, length int) []byte {
	b := n.Bytes()
	if len(b) >= length {
		return b
	}

	out := make([]byte, length)
	copy(out[length-len(b):], b)
	return out
}

// randomPrivate returns 32 cryptographically random bytes for the client's
// SRP private value 'a'.
func randomPrivate() ([]byte, error) {
	out := make([]byte, 32)
	if _, err := rand.Read(out); err != nil {
		return nil, err
	}

	return out, nil
}

// computeA returns A = g^a mod N, padded to length bytes.
func computeA(a []byte, n, g *big.Int, length int) []byte {
	A := new(big.Int).Exp(g, new(big.Int).SetBytes(a), n)
	return padTo(A, length)
}

// computeK returns the SRP multiplier k = SHA256(N_padded || g_padded).
func computeK(nPadded, gPad []byte) []byte {
	h := sha256.New()
	h.Write(nPadded)
	h.Write(gPad)
	return h.Sum(nil)
}

// computeU returns the scrambling parameter u = SHA256(A_padded || B_padded).
func computeU(aPadded, bPadded []byte) []byte {
	h := sha256.New()
	h.Write(aPadded)
	h.Write(bPadded)
	return h.Sum(nil)
}

// AppleS2K derives the SRP private value x using Apple's "s2k" construction:
//
//	p  = PBKDF2-HMAC-SHA256( SHA256(password), salt, iterations, 32 )
//	h2 = SHA256( ":" || p )
//	x  = SHA256( salt || h2 )
//
// With fo set, the password hash is hex-encoded before PBKDF2 ("s2k_fo").
// The email is intentionally absent: it only appears as H(email) inside M1.
func AppleS2K(password string, salt []byte, iterations int, fo bool) []byte {
	hashed := sha256.Sum256([]byte(password))

	var p []byte
	if fo {
		p = []byte(hex.EncodeToString(hashed[:]))
	} else {
		p = hashed[:]
	}

	p = pbkdf2.Key(p, salt, iterations, 32, sha256.New)

	h2 := sha256.Sum256(append([]byte(":"), p...))
	x := sha256.Sum256(append(append([]byte{}, salt...), h2[:]...))
	return x[:]
}

// computePremasterSecret returns the premaster secret
//
//	S = (B - k*g^x) ^ (a + u*x) mod N
//
// padded to length bytes. g is the unpadded generator value (2).
func computePremasterSecret(bPadded, k, gBytes, x, n, a, u []byte, length int) []byte {
	N := new(big.Int).SetBytes(n)
	G := new(big.Int).SetBytes(gBytes)
	B := new(big.Int).SetBytes(bPadded)
	K := new(big.Int).SetBytes(k)
	X := new(big.Int).SetBytes(x)
	A := new(big.Int).SetBytes(a)
	U := new(big.Int).SetBytes(u)

	// kv = k * g^x mod N
	kv := new(big.Int).Exp(G, X, N)
	kv.Mul(K, kv)
	kv.Mod(kv, N)

	// Bkv = (B - kv) mod N
	bkv := new(big.Int).Sub(B, kv)
	bkv.Mod(bkv, N)

	// exp = a + u*x
	exp := new(big.Int).Mul(U, X)
	exp.Add(A, exp)

	S := new(big.Int).Exp(bkv, exp, N)
	return padTo(S, length)
}

// sessionKey returns the session key K = SHA256(S_padded).
func sessionKey(sPadded []byte) []byte {
	h := sha256.Sum256(sPadded)
	return h[:]
}

// computeM1 returns the client evidence message:
//
//	M1 = SHA256( (H(N) XOR H(g)) || H(email_lowercase) || salt || A || B || K )
func computeM1(nPadded, gPad, emailLower, salt, aPadded, bPadded, k []byte) []byte {
	hN := sha256.Sum256(nPadded)
	hG := sha256.Sum256(gPad)

	xor := make([]byte, 32)
	for i := range xor {
		xor[i] = hN[i] ^ hG[i]
	}

	hEmail := sha256.Sum256(emailLower)

	h := sha256.New()
	h.Write(xor)
	h.Write(hEmail[:])
	h.Write(salt)
	h.Write(aPadded)
	h.Write(bPadded)
	h.Write(k)
	return h.Sum(nil)
}

// SPDKeyIV derives the AES-256-CBC key and IV used to decrypt the GSA "spd"
// payload from the SRP session key:
//
//	key = HMAC-SHA256(K, "extra data key:")
//	iv  = HMAC-SHA256(K, "extra data iv:")[:16]
func SPDKeyIV(k []byte) (key, iv []byte) {
	key = hmacSHA256(k, []byte("extra data key:"))
	iv = hmacSHA256(k, []byte("extra data iv:"))[:16]
	return key, iv
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// Proof is the client's side of an SRP-6a exchange.
type Proof struct {
	A  []byte // client public value A = g^a mod N, padded to GroupLength
	X  []byte // the 32-byte SRP private value x
	S  []byte // premaster secret, padded to GroupLength
	K  []byte // session key K = SHA256(S)
	M1 []byte // client evidence message
}

// Prove runs the client side of the SRP-6a exchange against the server values
// returned by the GSA "init" step. It uses Apple's "s2k" password derivation.
// If a is nil a fresh private value is generated; callers (tests) may pass a
// fixed value for reproducible output.
func Prove(email, password string, salt []byte, iterations int, fo bool, bPadded, a []byte) (*Proof, error) {
	if a == nil {
		var err error
		a, err = randomPrivate()
		if err != nil {
			return nil, err
		}
	}

	N := GroupN()
	G := GroupG()
	nPadded := padTo(N, GroupLength)
	gPad := gPadded(GroupLength)

	A := computeA(a, N, G, GroupLength)
	k := computeK(nPadded, gPad)
	u := computeU(A, bPadded)
	x := AppleS2K(password, salt, iterations, fo)
	S := computePremasterSecret(bPadded, k, G.Bytes(), x, nPadded, a, u, GroupLength)
	K := sessionKey(S)
	M1 := computeM1(nPadded, gPad, []byte(strings.ToLower(email)), salt, A, bPadded, K)

	return &Proof{
		A:  A,
		X:  x,
		S:  S,
		K:  K,
		M1: M1,
	}, nil
}

func mustHexDecode(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}
