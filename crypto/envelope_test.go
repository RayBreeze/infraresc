package crypto

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	plaintext := []byte("snapshot payload")
	sealed, err := Seal(plaintext, "correct horse battery staple")
	if err != nil { t.Fatalf("Seal: %v", err) }
	opened, err := Open(sealed, "correct horse battery staple")
	if err != nil { t.Fatalf("Open: %v", err) }
	if !bytes.Equal(opened, plaintext) { t.Fatalf("Open = %q, want %q", opened, plaintext) }
}

func TestOpenRejectsWrongPasswordAndTampering(t *testing.T) {
	sealed, err := Seal([]byte("snapshot payload"), "right-password")
	if err != nil { t.Fatalf("Seal: %v", err) }
	if _, err := Open(sealed, "wrong-password"); err == nil { t.Fatal("expected wrong password to fail") }

	var envelope Envelope
	if err := json.Unmarshal(sealed, &envelope); err != nil { t.Fatalf("unmarshal envelope: %v", err) }
	envelope.Ciphertext = envelope.Ciphertext[:len(envelope.Ciphertext)-1] + "A"
	tampered, err := json.Marshal(envelope)
	if err != nil { t.Fatalf("marshal tampered envelope: %v", err) }
	if _, err := Open(tampered, "right-password"); err == nil { t.Fatal("expected tampered ciphertext to fail authentication") }
}

func TestOpenRejectsUnsupportedEnvelopeMetadata(t *testing.T) {
	sealed, err := Seal([]byte("snapshot payload"), "pass")
	if err != nil { t.Fatalf("Seal: %v", err) }
	cases := []struct{name string; mutate func(*Envelope)}{
		{"version", func(e *Envelope) { e.Version = "999" }},
		{"algorithm", func(e *Envelope) { e.Algorithm = "other" }},
		{"kdf", func(e *Envelope) { e.KDF = "other" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var envelope Envelope
			if err := json.Unmarshal(sealed, &envelope); err != nil { t.Fatalf("unmarshal envelope: %v", err) }
			tc.mutate(&envelope)
			mutated, err := json.Marshal(envelope)
			if err != nil { t.Fatalf("marshal envelope: %v", err) }
			if _, err := Open(mutated, "pass"); err == nil { t.Fatal("expected unsupported metadata to be rejected") }
		})
	}
}

func TestSealRejectsEmptyPassword(t *testing.T) {
	if _, err := Seal([]byte("payload"), ""); err == nil { t.Fatal("expected empty password to be rejected") }
	if _, err := Open([]byte("{}"), ""); err == nil { t.Fatal("expected empty password to be rejected") }
}
