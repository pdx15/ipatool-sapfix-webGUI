package gsa

import (
	"testing"
)

// TestCBCDecrypt verifies AES-256-CBC spd decryption against an independently
// generated vector (see scripts/gen_gsa_vectors.py).
func TestCBCDecrypt(t *testing.T) {
	key := mustHexDecode("35b04773279d17bf79c4a9e29764c166cf18d0a9872d9d5d68f1a30120e8b5e3")
	iv := mustHexDecode("4110daa96bce99edc72635afcb3ee8f5")
	ciphertext := mustHexDecode("0224214c3b2fbc85ae693cf1f21926a45bd30e7abf31ed5a3440fd6282401091c9a10bd5a41ff812bb15416b9da1132759631bb05530c2b8073dd5a1dccee49996c78e5dca1673eb361fa608ad75a6ddb538ac169f23d690db4cb30963fbb21ece459e36279abeac60da6d04ce65551555c70e325ce73ce1981b2cd9e7259e4d")

	const want = `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<plist version="1.0"><dict><key>foo</key><string>bar</string></dict></plist>`

	plaintext, err := cbcDecrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("cbcDecrypt: %v", err)
	}

	if string(plaintext) != want {
		t.Fatalf("plaintext mismatch:\n got %q\nwant %q", string(plaintext), want)
	}
}

// TestCBCDecryptRejectsCorruption ensures invalid padding (wrong key) is
// reported rather than silently returning garbage.
func TestCBCDecryptRejectsCorruption(t *testing.T) {
	key := mustHexDecode("35b04773279d17bf79c4a9e29764c166cf18d0a9872d9d5d68f1a30120e8b5e3")
	iv := mustHexDecode("4110daa96bce99edc72635afcb3ee8f5")
	ciphertext := mustHexDecode("0224214c3b2fbc85ae693cf1f21926a45bd30e7abf31ed5a3440fd6282401091c9a10bd5a41ff812bb15416b9da1132759631bb05530c2b8073dd5a1dccee49996c78e5dca1673eb361fa608ad75a6ddb538ac169f23d690db4cb30963fbb21ece459e36279abeac60da6d04ce65551555c70e325ce73ce1981b2cd9e7259e4d")

	// Corrupt a byte inside the final (padding) block so PKCS#7 verification
	// fails. Flipping the first block would only garble the plaintext without
	// touching the padding, so it must be the last block here.
	ciphertext[len(ciphertext)-2] ^= 0xFF

	if _, err := cbcDecrypt(key, iv, ciphertext); err == nil {
		t.Fatal("expected an error for corrupted ciphertext")
	}
}
