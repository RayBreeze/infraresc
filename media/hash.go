package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

func SHA256File(path string) (string, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf(
			"reading artifact for hashing: %w",
			err,
		)
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(
		hash[:],
	), nil
}
