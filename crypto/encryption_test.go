package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte(`{"resourceType":"EC2","resourceId":"i-example"}`)
	passphrase := []byte("correct horse battery staple")
	aad := []byte("snapshot-id:cv-2026-09-16-001")

	blob, err := EncryptWithPassphrase(plaintext, passphrase, aad)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}

	got, err := DecryptWithPassphrase(blob, passphrase, aad)
	if err != nil {
		t.Fatalf("DecryptWithPassphrase: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestDecryptWrongPassphraseFails(t *testing.T) {
	blob, err := EncryptWithPassphrase([]byte("secret"), []byte("right-pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	if _, err := DecryptWithPassphrase(blob, []byte("wrong-pass"), nil); err == nil {
		t.Fatal("expected decryption to fail with wrong passphrase, got nil error")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	blob, err := EncryptWithPassphrase([]byte("secret data"), []byte("pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	blob.Ciphertext[0] ^= 0xFF // flip a bit
	if _, err := DecryptWithPassphrase(blob, []byte("pass"), nil); err == nil {
		t.Fatal("expected decryption to fail with tampered ciphertext, got nil error")
	}
}

func TestDecryptTamperedAdditionalDataFails(t *testing.T) {
	blob, err := EncryptWithPassphrase([]byte("secret data"), []byte("pass"), []byte("aad-v1"))
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	if _, err := DecryptWithPassphrase(blob, []byte("pass"), []byte("aad-v2")); err == nil {
		t.Fatal("expected decryption to fail with mismatched additional data, got nil error")
	}
}

func TestEncryptProducesUniqueNoncesAndCiphertexts(t *testing.T) {
	passphrase := []byte("pass")
	plaintext := []byte("same plaintext every time")

	blob1, err := EncryptWithPassphrase(plaintext, passphrase, nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	blob2, err := EncryptWithPassphrase(plaintext, passphrase, nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}

	if bytes.Equal(blob1.Nonce, blob2.Nonce) {
		t.Fatal("expected distinct nonces across encryptions")
	}
	if bytes.Equal(blob1.Ciphertext, blob2.Ciphertext) {
		t.Fatal("expected distinct ciphertexts across encryptions (different salt+nonce)")
	}
	if bytes.Equal(blob1.KDF.Salt, blob2.KDF.Salt) {
		t.Fatal("expected distinct salts across encryptions")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	params, err := NewKDFParams()
	if err != nil {
		t.Fatalf("NewKDFParams: %v", err)
	}
	k1, err := DeriveKey([]byte("pass"), params)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	k2, err := DeriveKey([]byte("pass"), params)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("expected same passphrase+params to derive identical keys")
	}
	if len(k1) != 32 {
		t.Fatalf("expected 32-byte key for AES-256, got %d", len(k1))
	}
}

func TestDeriveKeyRejectsEmptyPassphrase(t *testing.T) {
	params, err := NewKDFParams()
	if err != nil {
		t.Fatalf("NewKDFParams: %v", err)
	}
	if _, err := DeriveKey(nil, params); err == nil {
		t.Fatal("expected error for empty passphrase")
	}
}

// TestDeriveKeyRejectsMaliciousParams models an attacker who has modified
// the KDF parameters stored in an untrusted vault envelope, trying to turn
// "unlock this vault" into a memory- or CPU-exhaustion denial of service
// (or, for the zero/invalid cases, a broken/degenerate key). Every one of
// these must be rejected before Argon2id ever runs.
func TestDeriveKeyRejectsMaliciousParams(t *testing.T) {
	validSalt := func() []byte {
		p, err := NewKDFParams()
		if err != nil {
			t.Fatalf("NewKDFParams: %v", err)
		}
		return p.Salt
	}()

	cases := []struct {
		name   string
		params KDFParams
	}{
		{
			name: "memory way above cap",
			params: KDFParams{
				Salt: validSalt, Time: DefaultArgon2Time,
				Memory: MaxArgon2MemoryK * 100, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "zero time/iterations",
			params: KDFParams{
				Salt: validSalt, Time: 0,
				Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "time way above cap",
			params: KDFParams{
				Salt: validSalt, Time: MaxArgon2Time * 50,
				Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "parallelism way above cap",
			params: KDFParams{
				Salt: validSalt, Time: DefaultArgon2Time,
				Memory: DefaultArgon2MemoryK, Threads: 255, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "zero threads",
			params: KDFParams{
				Salt: validSalt, Time: DefaultArgon2Time,
				Memory: DefaultArgon2MemoryK, Threads: 0, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "salt too short",
			params: KDFParams{
				Salt: []byte{1, 2, 3}, Time: DefaultArgon2Time,
				Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "salt too long",
			params: KDFParams{
				Salt: make([]byte, 1024), Time: DefaultArgon2Time,
				Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen,
			},
		},
		{
			name: "wrong key length",
			params: KDFParams{
				Salt: validSalt, Time: DefaultArgon2Time,
				Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: 9999,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DeriveKey([]byte("some passphrase"), tc.params); err == nil {
				t.Fatalf("expected DeriveKey to reject params (%s), got nil error", tc.name)
			}
		})
	}
}

func TestDecryptRejectsMaliciousEmbeddedParams(t *testing.T) {
	// End-to-end version of the above: an attacker who tampers with the
	// KDF parameters inside an otherwise-real EncryptedBlob must not be
	// able to trigger an expensive Argon2id run via DecryptWithPassphrase.
	blob, err := EncryptWithPassphrase([]byte("secret"), []byte("pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	blob.KDF.Memory = MaxArgon2MemoryK * 100

	if _, err := DecryptWithPassphrase(blob, []byte("pass"), nil); err == nil {
		t.Fatal("expected DecryptWithPassphrase to reject an envelope with out-of-bounds KDF memory")
	}
}

// TestValidateKDFParamsAcceptsBoundaryValues checks the Min/Max bounds are
// inclusive exactly as intended: a value sitting exactly on a boundary
// must be accepted, not off-by-one rejected.
func TestValidateKDFParamsAcceptsBoundaryValues(t *testing.T) {
	salt := make([]byte, SaltSize)

	cases := []struct {
		name   string
		params KDFParams
	}{
		{"min time", KDFParams{Salt: salt, Time: MinArgon2Time, Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"max time", KDFParams{Salt: salt, Time: MaxArgon2Time, Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"min memory", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: MinArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"max memory", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: MaxArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"min threads", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: DefaultArgon2MemoryK, Threads: MinArgon2Threads, KeyLen: Argon2KeyLen}},
		{"max threads", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: DefaultArgon2MemoryK, Threads: MaxArgon2Threads, KeyLen: Argon2KeyLen}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateKDFParams(tc.params); err != nil {
				t.Fatalf("expected boundary value to be accepted, got error: %v", err)
			}
		})
	}
}

// TestValidateKDFParamsRejectsJustOutsideBoundaryValues is the complement
// of the above: one unit past each bound must still be rejected, proving
// the bounds are enforced precisely rather than approximately.
func TestValidateKDFParamsRejectsJustOutsideBoundaryValues(t *testing.T) {
	salt := make([]byte, SaltSize)

	cases := []struct {
		name   string
		params KDFParams
	}{
		{"just below min time", KDFParams{Salt: salt, Time: MinArgon2Time - 1, Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"just above max time", KDFParams{Salt: salt, Time: MaxArgon2Time + 1, Memory: DefaultArgon2MemoryK, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"just below min memory", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: MinArgon2MemoryK - 1, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"just above max memory", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: MaxArgon2MemoryK + 1, Threads: DefaultArgon2Threads, KeyLen: Argon2KeyLen}},
		{"just below min threads", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: DefaultArgon2MemoryK, Threads: MinArgon2Threads - 1, KeyLen: Argon2KeyLen}},
		{"just above max threads", KDFParams{Salt: salt, Time: DefaultArgon2Time, Memory: DefaultArgon2MemoryK, Threads: MaxArgon2Threads + 1, KeyLen: Argon2KeyLen}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateKDFParams(tc.params); err == nil {
				t.Fatalf("expected value just outside the boundary to be rejected, got nil error")
			}
		})
	}
}

// TestDecryptRejectsMalformedStructureBeforeKeyDerivation confirms that
// cheap structural problems (bad nonce length, truncated ciphertext) are
// caught by validateEncryptedBlobStructure and never reach DeriveKey. We
// can't directly assert "Argon2id didn't run" from outside the package,
// but these cases return quickly and with a structural error message,
// which is the observable behavior that matters.
func TestDecryptRejectsMalformedStructureBeforeKeyDerivation(t *testing.T) {
	validBlob, err := EncryptWithPassphrase([]byte("secret"), []byte("pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}

	t.Run("wrong nonce length", func(t *testing.T) {
		blob := *validBlob
		blob.Nonce = []byte{1, 2, 3}
		if _, err := DecryptWithPassphrase(&blob, []byte("pass"), nil); err == nil {
			t.Fatal("expected rejection for wrong nonce length")
		}
	})

	t.Run("truncated ciphertext", func(t *testing.T) {
		blob := *validBlob
		blob.Ciphertext = []byte{1, 2, 3} // shorter than a GCM tag
		if _, err := DecryptWithPassphrase(&blob, []byte("pass"), nil); err == nil {
			t.Fatal("expected rejection for ciphertext shorter than a GCM tag")
		}
	})

	t.Run("nil blob", func(t *testing.T) {
		if _, err := DecryptWithPassphrase(nil, []byte("pass"), nil); err == nil {
			t.Fatal("expected rejection for nil blob")
		}
	})
}

func TestEncryptSetsVersionAndAlgorithm(t *testing.T) {
	blob, err := EncryptWithPassphrase([]byte("secret"), []byte("pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	if blob.Version != EncryptionFormatVersion {
		t.Fatalf("expected version %d, got %d", EncryptionFormatVersion, blob.Version)
	}
	if blob.Algorithm != AlgorithmAES256GCM {
		t.Fatalf("expected algorithm %q, got %q", AlgorithmAES256GCM, blob.Algorithm)
	}
}

func TestDecryptRejectsUnknownVersion(t *testing.T) {
	blob, err := EncryptWithPassphrase([]byte("secret"), []byte("pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	blob.Version = 99 // a version this build has never heard of
	if _, err := DecryptWithPassphrase(blob, []byte("pass"), nil); err == nil {
		t.Fatal("expected DecryptWithPassphrase to reject an unknown format version")
	}
}

func TestDecryptRejectsUnknownAlgorithm(t *testing.T) {
	blob, err := EncryptWithPassphrase([]byte("secret"), []byte("pass"), nil)
	if err != nil {
		t.Fatalf("EncryptWithPassphrase: %v", err)
	}
	blob.Algorithm = "ChaCha20-Poly1305" // not what this build implements
	if _, err := DecryptWithPassphrase(blob, []byte("pass"), nil); err == nil {
		t.Fatal("expected DecryptWithPassphrase to reject an unrecognized algorithm")
	}
}
