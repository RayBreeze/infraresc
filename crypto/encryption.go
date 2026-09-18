package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Tunable Argon2id cost parameters. These are the *defaults* used for new
// vaults; every derived-key envelope also stores the parameters that were
// actually used, so changing these constants in a future release never
// breaks the ability to decrypt older vaults.
//
// The chosen values follow the OWASP / RFC 9106 "if you have the memory"
// recommendation for interactive-but-infrequent use (unlocking a recovery
// vault is a rare, deliberate operation, so we can afford to spend real
// time and memory to make offline brute-force expensive).
//
// These defaults are a starting point, not a benchmarked final answer:
// before shipping, measure actual unlock latency and peak RSS for
// DefaultArgon2Time/MemoryK/Threads on the least capable machine InfraKey
// is expected to run recovery on (e.g. a modest emergency-recovery laptop,
// not a beefy dev machine), and adjust the defaults — not the Min/Max
// bounds below — if it's too slow or memory-hungry there.
const (
	DefaultArgon2Time    uint32 = 3          // number of passes over memory
	DefaultArgon2MemoryK uint32 = 256 * 1024 // memory in KiB (256 MiB)
	DefaultArgon2Threads uint8  = 4          // parallelism
	Argon2KeyLen         uint32 = 32         // 256-bit key, for AES-256

	SaltSize  = 16 // bytes, required exact length for Argon2 salts here
	NonceSize = 12 // bytes, standard GCM nonce size

	// Bounds enforced on KDF parameters read back from a vault envelope.
	// A vault is untrusted input: an attacker who can modify the envelope
	// (e.g. a tampered USB artifact) could otherwise set an enormous
	// memory/time/parallelism value and turn a routine "unlock" into a
	// memory-exhaustion or CPU-exhaustion denial of service on whatever
	// machine tries to open it. These caps are deliberately generous for
	// legitimate use (well above DefaultArgon2*) but bounded.
	MaxArgon2Time    uint32 = 20
	MaxArgon2MemoryK uint32 = 1024 * 1024 // 1 GiB
	MaxArgon2Threads uint8  = 16
	MinArgon2Time    uint32 = 1
	MinArgon2MemoryK uint32 = 8 * 1024 // 8 MiB
	MinArgon2Threads uint8  = 1
)

// Format identifiers for EncryptedBlob. These exist so that a future
// InfraKey version can change the encryption construction (e.g. a
// different AEAD, or a hardware-backed KDF) without ambiguity about how
// to read an older vault: the blob says up front which version and
// algorithm produced it, and DecryptWithPassphrase refuses anything it
// doesn't explicitly know how to handle rather than guessing.
const (
	EncryptionFormatVersion uint8  = 1
	AlgorithmAES256GCM      string = "AES-256-GCM"
)

// KDFParams captures the Argon2id parameters used to derive a key, along
// with the random salt. Storing this next to the ciphertext means the vault
// is self-describing: InfraKey never has to guess which parameters were
// used to unlock an older snapshot.
type KDFParams struct {
	Salt    []byte `json:"salt"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memoryKiB"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

// NewKDFParams generates a fresh random salt and returns a KDFParams struct
// populated with the current default Argon2id cost parameters.
func NewKDFParams() (KDFParams, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return KDFParams{}, fmt.Errorf("crypto: generating salt: %w", err)
	}
	return KDFParams{
		Salt:    salt,
		Time:    DefaultArgon2Time,
		Memory:  DefaultArgon2MemoryK,
		Threads: DefaultArgon2Threads,
		KeyLen:  Argon2KeyLen,
	}, nil
}

// ValidateKDFParams bounds-checks params against the Min/Max Argon2*
// constants and rejects anything outside them. It must be called on any
// KDFParams read from an untrusted source (a vault envelope) before they
// are ever passed to Argon2id — the envelope is attacker-controlled input,
// and Argon2's cost parameters directly control how much memory and CPU
// the process will spend, so unbounded parameters are a denial-of-service
// vector, not just a correctness concern.
func ValidateKDFParams(params KDFParams) error {
	if len(params.Salt) != SaltSize {
		return fmt.Errorf("crypto: salt must be exactly %d bytes, got %d", SaltSize, len(params.Salt))
	}
	if params.Time < MinArgon2Time || params.Time > MaxArgon2Time {
		return fmt.Errorf("crypto: time parameter %d out of allowed range [%d, %d]", params.Time, MinArgon2Time, MaxArgon2Time)
	}
	if params.Memory < MinArgon2MemoryK || params.Memory > MaxArgon2MemoryK {
		return fmt.Errorf("crypto: memory parameter %dKiB out of allowed range [%d, %d]KiB", params.Memory, MinArgon2MemoryK, MaxArgon2MemoryK)
	}
	if params.Threads < MinArgon2Threads || params.Threads > MaxArgon2Threads {
		return fmt.Errorf("crypto: threads parameter %d out of allowed range [%d, %d]", params.Threads, MinArgon2Threads, MaxArgon2Threads)
	}
	if params.KeyLen != Argon2KeyLen {
		return fmt.Errorf("crypto: keyLen must be exactly %d (AES-256), got %d", Argon2KeyLen, params.KeyLen)
	}
	return nil
}

// DeriveKey runs Argon2id over passphrase using the given parameters and
// returns a symmetric key suitable for AES-256-GCM.
//
// params is always validated against ValidateKDFParams before use — this
// function is the single choke point through which vault-supplied KDF
// parameters reach Argon2id, so there is no path that skips bounds
// checking, including when params came from an untrusted envelope.
//
// The passphrase byte slice is not wiped by this function; callers that
// need best-effort secret hygiene should zero it themselves once done.
func DeriveKey(passphrase []byte, params KDFParams) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("crypto: passphrase must not be empty")
	}
	if params.KeyLen == 0 {
		params.KeyLen = Argon2KeyLen
	}
	if err := ValidateKDFParams(params); err != nil {
		return nil, fmt.Errorf("crypto: refusing to derive key: %w", err)
	}
	key := argon2.IDKey(passphrase, params.Salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return key, nil
}

// EncryptedBlob is the self-contained, on-disk representation of a piece of
// encrypted vault data: which format version and algorithm produced it,
// the KDF parameters needed to re-derive the key from a passphrase, and
// the AES-256-GCM nonce and ciphertext (which includes the GCM
// authentication tag).
//
// Version and Algorithm are deliberately explicit rather than implied by
// "whatever this code version happens to do" — a vault written today must
// still say what it is when read by InfraKey five versions from now.
type EncryptedBlob struct {
	Version    uint8     `json:"version"`
	Algorithm  string    `json:"algorithm"`
	KDF        KDFParams `json:"kdf"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
}

// EncryptWithPassphrase derives a key from passphrase using fresh KDF
// parameters and encrypts plaintext with AES-256-GCM. additionalData, if
// non-nil, is authenticated but not encrypted (e.g. a snapshot ID or
// manifest version) — tampering with it will cause decryption to fail.
func EncryptWithPassphrase(plaintext, passphrase, additionalData []byte) (*EncryptedBlob, error) {
	params, err := NewKDFParams()
	if err != nil {
		return nil, err
	}
	key, err := DeriveKey(passphrase, params)
	if err != nil {
		return nil, err
	}
	defer zero(key)

	ciphertext, nonce, err := encryptAESGCM(plaintext, key, additionalData)
	if err != nil {
		return nil, err
	}

	return &EncryptedBlob{
		Version:    EncryptionFormatVersion,
		Algorithm:  AlgorithmAES256GCM,
		KDF:        params,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// DecryptWithPassphrase re-derives the key from passphrase using the KDF
// parameters stored in blob, then decrypts and authenticates the
// ciphertext. It returns an error (rather than garbage plaintext) if the
// passphrase is wrong, the blob/additionalData has been tampered with, or
// the blob declares a format version/algorithm this build doesn't
// implement — an explicit "I don't know how to read this" is far safer
// than silently applying today's decryption logic to a blob that was
// written under different assumptions.
//
// All cheap structural checks (version, algorithm, salt/nonce/ciphertext
// lengths, KDF parameter bounds) run before Argon2id is ever invoked. blob
// comes from untrusted recovery media, and Argon2id with the configured
// memory/time cost is by design an expensive operation — a malformed or
// hostile blob should be rejected on cheap checks alone, not after paying
// for a full key derivation first.
func DecryptWithPassphrase(blob *EncryptedBlob, passphrase, additionalData []byte) ([]byte, error) {
	if err := validateEncryptedBlobStructure(blob); err != nil {
		return nil, fmt.Errorf("crypto: rejecting malformed vault data before key derivation: %w", err)
	}

	key, err := DeriveKey(passphrase, blob.KDF)
	if err != nil {
		return nil, err
	}
	defer zero(key)

	plaintext, err := decryptAESGCM(blob.Ciphertext, key, blob.Nonce, additionalData)
	if err != nil {
		// Deliberately vague: don't reveal whether the passphrase, the
		// ciphertext, or the additional data was the problem.
		return nil, errors.New("crypto: decryption failed (wrong passphrase or corrupted/tampered vault data)")
	}
	return plaintext, nil
}

// minGCMCiphertextSize is the smallest a GCM ciphertext can legitimately
// be: even encrypting zero bytes of plaintext still produces a full
// 16-byte authentication tag.
const minGCMCiphertextSize = 16

// validateEncryptedBlobStructure performs all the cheap, purely structural
// checks on blob — the ones that don't require deriving a key or touching
// the AEAD — so that malformed or hostile input is rejected before any
// expensive cryptographic work happens. It intentionally does not attempt
// decryption; that's still decryptAESGCM's job.
func validateEncryptedBlobStructure(blob *EncryptedBlob) error {
	if blob == nil {
		return errors.New("blob must not be nil")
	}
	if blob.Version != EncryptionFormatVersion {
		return fmt.Errorf("unsupported encryption format version %d (this build supports version %d)", blob.Version, EncryptionFormatVersion)
	}
	if blob.Algorithm != AlgorithmAES256GCM {
		return fmt.Errorf("unsupported algorithm %q (this build supports %q)", blob.Algorithm, AlgorithmAES256GCM)
	}
	if err := ValidateKDFParams(blob.KDF); err != nil {
		return fmt.Errorf("invalid KDF parameters: %w", err)
	}
	if len(blob.Nonce) != NonceSize {
		return fmt.Errorf("nonce must be exactly %d bytes, got %d", NonceSize, len(blob.Nonce))
	}
	if len(blob.Ciphertext) < minGCMCiphertextSize {
		return fmt.Errorf("ciphertext too short to contain a GCM tag: got %d bytes, need at least %d", len(blob.Ciphertext), minGCMCiphertextSize)
	}
	return nil
}

// encryptAESGCM encrypts plaintext under key (must be 32 bytes for
// AES-256) using a freshly generated random nonce, returning the
// ciphertext (with appended GCM tag) and the nonce used.
func encryptAESGCM(plaintext, key, additionalData []byte) (ciphertext, nonce []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("crypto: generating nonce: %w", err)
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, additionalData)
	return ciphertext, nonce, nil
}

// decryptAESGCM decrypts and authenticates ciphertext under key and nonce.
func decryptAESGCM(ciphertext, key, nonce, additionalData []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("crypto: invalid nonce size: got %d, want %d", len(nonce), gcm.NonceSize())
	}
	return gcm.Open(nil, nonce, ciphertext, additionalData)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes for AES-256, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating GCM mode: %w", err)
	}
	return gcm, nil
}

// zero overwrites a byte slice with zeros. This is best-effort hygiene:
// the Go runtime/GC can still leave copies elsewhere in memory, but it's
// better than leaving key material sitting around indefinitely.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
