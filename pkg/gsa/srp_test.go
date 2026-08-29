package gsa

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestGroupConstants pins the RFC 5054 2048-bit group parameters so that a
// silent regression in the modulus or generator cannot slip through.
func TestGroupConstants(t *testing.T) {
	if got := hex.EncodeToString(GroupN().Bytes()); got != n2048Hex() {
		t.Fatalf("N mismatch:\n got %s\nwant %s", got, n2048Hex())
	}

	if GroupG().Int64() != 2 {
		t.Fatalf("g = %v, want 2", GroupG())
	}
}

// TestAppleS2K checks Apple's "s2k" password derivation against
// independently computed vectors (see scripts/gen_gsa_vectors.py).
func TestAppleS2K(t *testing.T) {
	tests := []struct {
		name     string
		password string
		salt     string
		iters    int
		fo       bool
		want     string
	}{
		{
			name:     "s2k",
			password: "password123",
			salt:     "beb25379d1a8581eb5a727673a2441ee",
			iters:    20000,
			fo:       false,
			want:     "0b7c3f679819c32ab6cd24220eb66d3d8b5aa06c600055902d32a5322445d31a",
		},
		{
			name:     "s2k_fo",
			password: "password123",
			salt:     "beb25379d1a8581eb5a727673a2441ee",
			iters:    20000,
			fo:       true,
			want:     "08001b8e334a2d95c1c625bac24904f71a973e0e2f4ec49091c71716da242420",
		},
		{
			name:     "s2k_second",
			password: "Test@pple!P4ss",
			salt:     "6f8a2c4e1b3d5a7f9c0e2d4b6a8c1e3f",
			iters:    11011,
			fo:       false,
			want:     "67afcf2f20969a858440443bf1d0aad033255791ce4daf99894b8572ce386f12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			salt := mustHexDecode(tt.salt)
			got := hex.EncodeToString(AppleS2K(tt.password, salt, tt.iters, tt.fo))
			if got != tt.want {
				t.Fatalf("x mismatch:\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestProve runs a complete SRP-6a client exchange against fixed inputs and
// checks A, u, S, K and M1 (vector generated independently with
// scripts/gen_gsa_vectors.py).
func TestProve(t *testing.T) {
	const (
		email    = "user@example.com"
		password = "correct horse battery staple"
	)

	salt := mustHexDecode("beb25379d1a8581eb5a727673a2441ee")
	a := mustHexDecode("60975527035cf2ad1989806f0407210bc81edc04e2762a56afd529ddda2d4393")
	B := mustHexDecode("410813e3063f3b4532f2d36413749f39c26c5ceeb1346d3995003c74544c30cba318f981281607ae68dbdc3bee9f0544ada6b13d8ac33217b670973152cf03ef03797615e81dd305342c2e3bb035321d1fd717952e702b09682102d0a5aa25dcee01784a32b0684f75626ca3bf8aec874f2dc11f8926944b06f9948e8ad7649025a58cd9dccdb6b210de00e2283e72baaf93a39b0417dfd1888f841f43d7d41c75b58f654ccb2e8b9c875c42edc34fd3796200312f2abd19b7e2c54b5702cd1a7f4d79fdf73bc418c96466ba122d45474ab6db553417715617f6c3b4a8764279f086acc655e396f85812c90f6f932ce0586168c5deccc9f8beb6891ad13f7caf")

	proof, err := Prove(email, password, salt, 20000, false, B, a)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	// Independent u = SHA256(A || B), verified against the generated vector.
	u := computeU(proof.A, B)
	if got, want := hex.EncodeToString(u), "d56e895d00cb8a9ea81f0c9967522018bca195a485cd59687ebb2a3f5ecda88b"; got != want {
		t.Fatalf("u mismatch:\n got %s\nwant %s", got, want)
	}

	want := map[string]string{
		"A":  "4b700f8d48e69c9aae40c684ac7c7c03121e2b7602eb4c3514804ccada0ed4019193a351ecc65a6f854ede91eb096e721b22d701c7adc64e9cedacd75f2e26bb2f5e45dd53dc8dbeafffe82aa49fca0573444691212537a73cf80e25039258205a7edf4749b30adaf25877c62fcd09d6613598bcd4baf2a9727a53706a278148992b2abb23ad5d512d269e16ca11bc0895b5a3b5ec4721cde40a8c39c796e94f0be86dbbeb33da7037018983921aba3f5053195d5ac1da4e567e3c0e75d9e0609f92e850657b2be4771f415b9cacc5c1ecedc30133bf6474f5022c6519d780760ca4d8d3b966b034bd73877c1b3b33f474b9c3c5299a1968f3e6cd3bfe84445a",
		"S":  "6b861b62bf454806cea529c28e26990cc401367e6a6ab397580d0133bbdad8e396f093ed93a26cf81c09eae56d90b33297a564f2b8e7bd1c1b30f737a97b72b08ef1104c6ace5fdb9771c28e1d36d2feb795463bc1c4819abc3315a2a65652ea02c16873f090c43a6fff4c5bd8767b4f20f8450995cd85fae1263530ad6be52b0961413c5fd4520881b6c8b9c8a5855cbc71650477b783baf8d71e721df50a2a4d2e8fdbbef9da65a9e42efb1b092ce8d447d9d70f2d5ed300cfdfbe7ee528e787017a64edde253543ac97c34ac7d2289585438ffba0c47a9719cf1fc389a3b0bb8071f5b10bd3007547c6e10a3715c95f4424f4d8030cb16b6bae2f6b817bb6",
		"K":  "9ad60ea5527fb88902f96151fbddf46a0533c0bc1421ddf8fce24e34dd6b1ffa",
		"M1": "e124409750c9a2658b3a79082739510cf88049e787a623ad0169652789b5e3d9",
		"X":  "8d5e7cd3d5b1093ef68d39278910a9c1583b1154d2d7a33eb3745613583e1601",
	}

	for field, want := range want {
		got := hex.EncodeToString(proofField(proof, field))
		if got != want {
			t.Fatalf("%s mismatch:\n got %s\nwant %s", field, got, want)
		}
	}
}

// TestSPDKeyIV checks the spd AES key/IV derivation against the generated
// vector (uses the same session key K as TestProve).
func TestSPDKeyIV(t *testing.T) {
	k := mustHexDecode("9ad60ea5527fb88902f96151fbddf46a0533c0bc1421ddf8fce24e34dd6b1ffa")
	key, iv := SPDKeyIV(k)

	if got, want := hex.EncodeToString(key), "35b04773279d17bf79c4a9e29764c166cf18d0a9872d9d5d68f1a30120e8b5e3"; got != want {
		t.Fatalf("spd key mismatch:\n got %s\nwant %s", got, want)
	}

	if got, want := hex.EncodeToString(iv), "4110daa96bce99edc72635afcb3ee8f5"; got != want {
		t.Fatalf("spd iv mismatch:\n got %s\nwant %s", got, want)
	}
}

func proofField(p *Proof, field string) []byte {
	switch field {
	case "A":
		return p.A
	case "X":
		return p.X
	case "S":
		return p.S
	case "K":
		return p.K
	case "M1":
		return p.M1
	}

	panic("unknown field " + field)
}

func n2048Hex() string {
	return "ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b855f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773bca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb694b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73"
}

// TestProveUsesRandomPrivate ensures Prove generates a private value when none
// is supplied and produces distinct proofs.
func TestProveUsesRandomPrivate(t *testing.T) {
	salt := mustHexDecode("beb25379d1a8581eb5a727673a2441ee")
	B := bytes.Repeat([]byte{0x01}, GroupLength)

	p1, err := Prove("user@example.com", "password", salt, 20000, false, B, nil)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	p2, err := Prove("user@example.com", "password", salt, 20000, false, B, nil)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	if bytes.Equal(p1.A, p2.A) {
		t.Fatal("expected distinct public values from distinct private values")
	}
}
