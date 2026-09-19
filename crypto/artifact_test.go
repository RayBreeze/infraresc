package crypto

import (
	"bytes"
	"testing"
)

func TestArtifactRoundTrip(t *testing.T) {
	payload := []byte("{"version":"1","resources":[]}")
	artifact, err := NewArtifact(payload, "123456789012", "ap-south-1", []byte("passphrase"))
	if err != nil { t.Fatalf("NewArtifact: %v", err) }
	if artifact.Format != ArtifactFormat || artifact.Version != ArtifactVersion { t.Fatalf("unexpected artifact metadata: %#v", artifact) }
	if artifact.AccountID != "123456789012" || artifact.Region != "ap-south-1" { t.Fatalf("unexpected artifact identity: %#v", artifact) }
	if artifact.SnapshotHash != HashBytes(payload) { t.Fatal("artifact snapshot hash does not match payload") }

	data, err := SerializeArtifact(artifact)
	if err != nil { t.Fatalf("SerializeArtifact: %v", err) }
	decoded, err := DeserializeArtifact(data)
	if err != nil { t.Fatalf("DeserializeArtifact: %v", err) }

	opened, err := decoded.Decrypt([]byte("passphrase"))
	if err != nil { t.Fatalf("Decrypt: %v", err) }
	if !bytes.Equal(opened, payload) { t.Fatalf("decrypted payload = %q, want %q", opened, payload) }
}

func TestArtifactRejectsEmptyPayloadAndUnsupportedMetadata(t *testing.T) {
	if _, err := NewArtifact(nil, "account", "region", []byte("pass")); err == nil { t.Fatal("expected empty payload to be rejected") }
	artifact, err := NewArtifact([]byte("payload"), "account", "region", []byte("pass"))
	if err != nil { t.Fatalf("NewArtifact: %v", err) }
	artifact.Version = "999"
	data, err := SerializeArtifact(artifact)
	if err != nil { t.Fatalf("SerializeArtifact: %v", err) }
	if _, err := DeserializeArtifact(data); err == nil { t.Fatal("expected unsupported artifact version to be rejected") }
}
