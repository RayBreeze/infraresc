package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
)

// KeyPair holds an Ed25519 signing key pair. PrivateKey must never be
// written to the recovery medium; only PublicKey (and signatures produced
// by PrivateKey) are ever stored in the vault.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair creates a new random Ed25519 key pair, used to sign
// snapshot manifests so that `infrakey verify` can detect tampering with
// vault contents or metadata.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("crypto: generating Ed25519 key pair: %w", err)
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

// Sign signs message with the key pair's private key and returns the raw
// 64-byte Ed25519 signature.
func (kp *KeyPair) Sign(message []byte) ([]byte, error) {
	if kp == nil || len(kp.PrivateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("crypto: invalid or missing private key")
	}
	return ed25519.Sign(kp.PrivateKey, message), nil
}

// Verify reports whether signature is a valid Ed25519 signature of message
// under publicKey.
func Verify(publicKey ed25519.PublicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(publicKey, message, signature)
}

// Format identifiers for SignedManifest, mirroring EncryptedBlob's
// Version/Algorithm tagging. If InfraKey ever moves off Ed25519 (e.g. to
// a post-quantum scheme), a SignedManifest read off an old vault must say
// so explicitly rather than being handed to a verifier that assumes
// today's algorithm.
const (
	SignatureFormatVersion uint8  = 1
	AlgorithmEd25519       string = "Ed25519"
)

// SignedManifest bundles a manifest's raw bytes with its signature and the
// public key that should be used to verify it. Storing the public key
// alongside the signature keeps the vault self-describing without needing
// a separate key-distribution step for the MVP — but see
// VerifySignedManifest's doc comment: this embedded public key is never
// itself the root of trust.
type SignedManifest struct {
	Version   uint8  `json:"version"`
	Algorithm string `json:"algorithm"`
	Manifest  []byte `json:"manifest"`
	Signature []byte `json:"signature"`
	PublicKey []byte `json:"publicKey"`
}

// SignManifest signs manifestBytes (the canonical serialized form of a
// snapshot manifest — see state/manifest.go) with kp and returns a
// SignedManifest ready to be written to the vault.
func SignManifest(manifestBytes []byte, kp *KeyPair) (*SignedManifest, error) {
	sig, err := kp.Sign(manifestBytes)
	if err != nil {
		return nil, err
	}
	return &SignedManifest{
		Version:   SignatureFormatVersion,
		Algorithm: AlgorithmEd25519,
		Manifest:  manifestBytes,
		Signature: sig,
		PublicKey: []byte(kp.PublicKey),
	}, nil
}

// VerifySignedManifest reports whether sm's signature is valid, checked
// against trustedPublicKey — a key the caller obtained independently of
// sm itself (e.g. recorded to local config at `infrakey init` time, or
// passed in on the command line).
//
// trustedPublicKey is mandatory (a nil key is an error). This is the whole
// point of a signature check on untrusted media: an artifact that carries
// its own manifest, its own signature, and its own "here's the public key
// that signed it" is trivially self-consistent — an attacker who can
// rewrite the vault can generate a fresh Ed25519 keypair, sign whatever
// manifest they like, and embed the matching public key, and this
// function would have no way to detect that if it trusted the embedded
// key. Verification is only meaningful when trustedPublicKey came from
// somewhere the attacker doesn't control.
//
// sm.Version and sm.Algorithm are also checked explicitly, so a vault
// written under a future signature scheme fails loudly here instead of
// being handed to ed25519.Verify with the wrong assumptions.
func VerifySignedManifest(sm *SignedManifest, trustedPublicKey ed25519.PublicKey) (bool, error) {
	if len(trustedPublicKey) != ed25519.PublicKeySize {
		return false, errors.New("crypto: trustedPublicKey is required and must be a valid Ed25519 public key (use VerifySignedManifestTOFU only for first-time setup)")
	}
	if sm == nil {
		return false, errors.New("crypto: signed manifest must not be nil")
	}
	if sm.Version != SignatureFormatVersion {
		return false, fmt.Errorf("crypto: unsupported signature format version %d (this build supports version %d)", sm.Version, SignatureFormatVersion)
	}
	if sm.Algorithm != AlgorithmEd25519 {
		return false, fmt.Errorf("crypto: unsupported signature algorithm %q (this build supports %q)", sm.Algorithm, AlgorithmEd25519)
	}
	if len(sm.PublicKey) != ed25519.PublicKeySize {
		return false, errors.New("crypto: embedded public key has invalid length")
	}
	pub := ed25519.PublicKey(sm.PublicKey)

	if !equalKeys(pub, trustedPublicKey) {
		return false, errors.New("crypto: manifest's embedded public key does not match trusted public key")
	}

	return Verify(pub, sm.Manifest, sm.Signature), nil
}

// VerifySignedManifestTOFU ("trust on first use") verifies sm's signature
// against whatever public key is embedded in it, with no independent
// pinned key to check against.
//
// This provides no protection against an attacker who controls the whole
// artifact — see VerifySignedManifest's doc comment. It exists only for
// the one moment where there is nothing else to trust yet: `infrakey init`
// generating a brand-new vault and recording its public key for the first
// time. Every later verification (`infrakey verify`, `infrakey recover`)
// must use VerifySignedManifest with that recorded key instead. Callers
// should treat a call to this function as a code-review flag.
func VerifySignedManifestTOFU(sm *SignedManifest) (valid bool, observedPublicKey ed25519.PublicKey, err error) {
	if sm == nil {
		return false, nil, errors.New("crypto: signed manifest must not be nil")
	}
	if sm.Version != SignatureFormatVersion {
		return false, nil, fmt.Errorf("crypto: unsupported signature format version %d (this build supports version %d)", sm.Version, SignatureFormatVersion)
	}
	if sm.Algorithm != AlgorithmEd25519 {
		return false, nil, fmt.Errorf("crypto: unsupported signature algorithm %q (this build supports %q)", sm.Algorithm, AlgorithmEd25519)
	}
	if len(sm.PublicKey) != ed25519.PublicKeySize {
		return false, nil, errors.New("crypto: embedded public key has invalid length")
	}
	pub := ed25519.PublicKey(sm.PublicKey)
	return Verify(pub, sm.Manifest, sm.Signature), pub, nil
}

func equalKeys(a, b ed25519.PublicKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
