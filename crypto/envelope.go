package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	FormatVersion = "1"

	KDFArgon2id = "Argon2id"

	saltSize = 16
	keySize  = 32

	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
)

type Envelope struct {
	Version   string `json:"version"`
	Algorithm string `json:"algorithm"`
	KDF       string `json:"kdf"`

	Salt  string `json:"salt"`
	Nonce string `json:"nonce"`

	Ciphertext string `json:"ciphertext"`
}

// Seal encrypts a snapshot payload.
//
// Password
//
//	↓
//
// Argon2id
//
//	↓
//
// 256-bit key
//
//	↓
//
// AES-256-GCM
func Seal(
	plaintext []byte,
	password string,
) ([]byte, error) {

	if password == "" {
		return nil, fmt.Errorf(
			"encryption password cannot be empty",
		)
	}

	salt := make(
		[]byte,
		saltSize,
	)

	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf(
			"generating encryption salt: %w",
			err,
		)
	}

	key := argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		keySize,
	)

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, fmt.Errorf(
			"creating AES cipher: %w",
			err,
		)
	}

	aead, err := cipher.NewGCM(block)

	if err != nil {
		return nil, fmt.Errorf(
			"creating GCM cipher: %w",
			err,
		)
	}

	nonce := make(
		[]byte,
		aead.NonceSize(),
	)

	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf(
			"generating encryption nonce: %w",
			err,
		)
	}

	// Authenticate the artifact format version.
	ciphertext := aead.Seal(
		nil,
		nonce,
		plaintext,
		[]byte(FormatVersion),
	)

	envelope := Envelope{
		Version:   FormatVersion,
		Algorithm: AlgorithmAES256GCM,
		KDF:       KDFArgon2id,

		Salt: base64.StdEncoding.EncodeToString(
			salt,
		),

		Nonce: base64.StdEncoding.EncodeToString(
			nonce,
		),

		Ciphertext: base64.StdEncoding.EncodeToString(
			ciphertext,
		),
	}

	data, err := json.MarshalIndent(
		envelope,
		"",
		"  ",
	)

	if err != nil {
		return nil, fmt.Errorf(
			"serializing encrypted envelope: %w",
			err,
		)
	}

	return data, nil
}

// Open decrypts an InfraResc encrypted artifact.
func Open(
	data []byte,
	password string,
) ([]byte, error) {

	if password == "" {
		return nil, fmt.Errorf(
			"decryption password cannot be empty",
		)
	}

	var envelope Envelope

	if err := json.Unmarshal(
		data,
		&envelope,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid InfraResc encrypted artifact: %w",
			err,
		)
	}

	if envelope.Version != FormatVersion {
		return nil, fmt.Errorf(
			"unsupported artifact version %q",
			envelope.Version,
		)
	}

	if envelope.Algorithm != AlgorithmAES256GCM {
		return nil, fmt.Errorf(
			"unsupported encryption algorithm %q",
			envelope.Algorithm,
		)
	}

	if envelope.KDF != KDFArgon2id {
		return nil, fmt.Errorf(
			"unsupported key derivation function %q",
			envelope.KDF,
		)
	}

	salt, err := base64.StdEncoding.DecodeString(
		envelope.Salt,
	)

	if err != nil ||
		len(salt) != saltSize {

		return nil, fmt.Errorf(
			"invalid encryption salt",
		)
	}

	nonce, err := base64.StdEncoding.DecodeString(
		envelope.Nonce,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"invalid encryption nonce",
		)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(
		envelope.Ciphertext,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"invalid encrypted payload",
		)
	}

	key := argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		keySize,
	)

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, fmt.Errorf(
			"creating AES cipher: %w",
			err,
		)
	}

	aead, err := cipher.NewGCM(block)

	if err != nil {
		return nil, fmt.Errorf(
			"creating GCM cipher: %w",
			err,
		)
	}

	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf(
			"invalid encryption nonce size",
		)
	}

	plaintext, err := aead.Open(
		nil,
		nonce,
		ciphertext,
		[]byte(FormatVersion),
	)

	if err != nil {
		return nil, fmt.Errorf(
			"decrypting artifact: authentication failed",
		)
	}

	return plaintext, nil
}
