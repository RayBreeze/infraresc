package crypto

import "testing"

func TestSignVerifyRoundTrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	message := []byte(`{"snapshotId":"cv-2026-09-16-001","resourceCount":184}`)

	sig, err := kp.Sign(message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !Verify(kp.PublicKey, message, sig) {
		t.Fatal("expected valid signature to verify")
	}
}

func TestVerifyRejectsTamperedMessage(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	message := []byte("original manifest bytes")
	sig, err := kp.Sign(message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	tampered := []byte("tampered manifest bytes")
	if Verify(kp.PublicKey, tampered, sig) {
		t.Fatal("expected verification to fail for tampered message")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	kp1, _ := GenerateKeyPair()
	kp2, _ := GenerateKeyPair()
	message := []byte("manifest")

	sig, err := kp1.Sign(message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if Verify(kp2.PublicKey, message, sig) {
		t.Fatal("expected verification to fail under a different public key")
	}
}

func TestSignedManifestRoundTrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	manifest := []byte(`{"snapshotId":"cv-2026-09-16-001"}`)

	sm, err := SignManifest(manifest, kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	// Real verification always requires an independently-obtained trusted
	// key; here that's simply the key pair's own public key, standing in
	// for "the key InfraKey recorded at init time".
	ok, err := VerifySignedManifest(sm, kp.PublicKey)
	if err != nil {
		t.Fatalf("VerifySignedManifest: %v", err)
	}
	if !ok {
		t.Fatal("expected signed manifest to verify")
	}
}

func TestSignedManifestDetectsTamperedManifest(t *testing.T) {
	kp, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("original"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	sm.Manifest = []byte("tampered")
	ok, err := VerifySignedManifest(sm, kp.PublicKey)
	if err != nil {
		t.Fatalf("VerifySignedManifest: %v", err)
	}
	if ok {
		t.Fatal("expected verification to fail after manifest tampering")
	}
}

func TestVerifySignedManifestRequiresTrustedKey(t *testing.T) {
	kp, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	// Calling with no trusted key must be a hard error, not a fallback to
	// "trust whatever key is embedded" — that fallback is exactly the
	// self-consistent-forgery hole a tampered artifact could exploit.
	if _, err := VerifySignedManifest(sm, nil); err == nil {
		t.Fatal("expected error when trustedPublicKey is nil")
	}
}

func TestSignedManifestPinnedKeyMismatch(t *testing.T) {
	kp, _ := GenerateKeyPair()
	other, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	// A vault claiming to be signed by a different (trusted) key than the
	// one actually embedded must be rejected, even though the embedded
	// signature+key are internally consistent. This models an attacker
	// who swaps both the manifest and the public key together.
	ok, err := VerifySignedManifest(sm, other.PublicKey)
	if err == nil && ok {
		t.Fatal("expected verification to fail when embedded key does not match trusted key")
	}
}

func TestVerifySignedManifestTOFU(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	sm, err := SignManifest([]byte("brand new vault, no trusted key recorded yet"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	ok, observedKey, err := VerifySignedManifestTOFU(sm)
	if err != nil {
		t.Fatalf("VerifySignedManifestTOFU: %v", err)
	}
	if !ok {
		t.Fatal("expected TOFU verification to succeed for a freshly signed manifest")
	}
	if !equalKeys(observedKey, kp.PublicKey) {
		t.Fatal("expected TOFU to return the embedded public key so callers can record it")
	}
}

func TestSignManifestSetsVersionAndAlgorithm(t *testing.T) {
	kp, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}
	if sm.Version != SignatureFormatVersion {
		t.Fatalf("expected version %d, got %d", SignatureFormatVersion, sm.Version)
	}
	if sm.Algorithm != AlgorithmEd25519 {
		t.Fatalf("expected algorithm %q, got %q", AlgorithmEd25519, sm.Algorithm)
	}
}

func TestVerifySignedManifestRejectsUnknownVersion(t *testing.T) {
	kp, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}
	sm.Version = 99
	if _, err := VerifySignedManifest(sm, kp.PublicKey); err == nil {
		t.Fatal("expected VerifySignedManifest to reject an unknown signature format version")
	}
}

func TestVerifySignedManifestRejectsUnknownAlgorithm(t *testing.T) {
	kp, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}
	sm.Algorithm = "Dilithium2"
	if _, err := VerifySignedManifest(sm, kp.PublicKey); err == nil {
		t.Fatal("expected VerifySignedManifest to reject an unrecognized signature algorithm")
	}
}

func TestVerifySignedManifestTOFURejectsUnknownVersion(t *testing.T) {
	kp, _ := GenerateKeyPair()
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}
	sm.Version = 99
	if _, _, err := VerifySignedManifestTOFU(sm); err == nil {
		t.Fatal("expected VerifySignedManifestTOFU to reject an unknown signature format version")
	}
}

// TestVerifyRejectsMalformedSignatureLengths makes explicit what
// ed25519.Verify already enforces internally: signatures and public keys
// of the wrong length must be rejected, not panic or silently coerce.
func TestVerifyRejectsMalformedSignatureLengths(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	message := []byte("manifest bytes")
	validSig, err := kp.Sign(message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	t.Run("empty signature", func(t *testing.T) {
		if Verify(kp.PublicKey, message, nil) {
			t.Fatal("expected empty signature to fail verification")
		}
	})
	t.Run("truncated signature", func(t *testing.T) {
		if Verify(kp.PublicKey, message, validSig[:len(validSig)-1]) {
			t.Fatal("expected truncated signature to fail verification")
		}
	})
	t.Run("oversized signature", func(t *testing.T) {
		oversized := append(append([]byte{}, validSig...), 0x00)
		if Verify(kp.PublicKey, message, oversized) {
			t.Fatal("expected oversized signature to fail verification")
		}
	})
	t.Run("wrong-length public key", func(t *testing.T) {
		if Verify(kp.PublicKey[:len(kp.PublicKey)-1], message, validSig) {
			t.Fatal("expected wrong-length public key to fail verification")
		}
	})
	t.Run("empty public key", func(t *testing.T) {
		if Verify(nil, message, validSig) {
			t.Fatal("expected empty public key to fail verification")
		}
	})
}

// TestSignedManifestRejectsMalformedEmbeddedPublicKey covers the
// higher-level SignedManifest path: a corrupted/truncated embedded public
// key must be caught explicitly, with a clear error, rather than reaching
// ed25519.Verify with bad input.
func TestSignedManifestRejectsMalformedEmbeddedPublicKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	sm, err := SignManifest([]byte("manifest"), kp)
	if err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	sm.PublicKey = sm.PublicKey[:len(sm.PublicKey)-1] // truncate by one byte
	if _, err := VerifySignedManifest(sm, kp.PublicKey); err == nil {
		t.Fatal("expected error for truncated embedded public key")
	}
	if _, _, err := VerifySignedManifestTOFU(sm); err == nil {
		t.Fatal("expected error for truncated embedded public key (TOFU path)")
	}
}
