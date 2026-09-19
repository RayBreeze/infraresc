package crypto

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	ArtifactFormat  = "infraresc-snapshot"
	ArtifactVersion = "1"
)

type Artifact struct {
	Format       string        `json:"format"`
	Version      string        `json:"version"`
	CreatedAt    time.Time     `json:"created_at"`
	AccountID    string        `json:"account_id"`
	Region       string        `json:"region"`
	SnapshotHash string        `json:"snapshot_hash"`
	Encrypted    EncryptedBlob `json:"encrypted"`
}

func NewArtifact(
	payload []byte,
	accountID string,
	region string,
	passphrase []byte,
) (*Artifact, error) {

	if len(payload) == 0 {
		return nil, fmt.Errorf("artifact payload is empty")
	}

	hash := HashBytes(payload)

	createdAt := time.Now().UTC()

	aad := []byte(fmt.Sprintf(
		"%s:%s:%s:%s:%s",
		ArtifactFormat,
		ArtifactVersion,
		accountID,
		region,
		hash,
	))

	encrypted, err := EncryptWithPassphrase(
		payload,
		passphrase,
		aad,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"encrypting artifact: %w",
			err,
		)
	}

	return &Artifact{
		Format:       ArtifactFormat,
		Version:      ArtifactVersion,
		CreatedAt:    createdAt,
		AccountID:    accountID,
		Region:       region,
		SnapshotHash: hash,
		Encrypted:    *encrypted,
	}, nil
}

func SerializeArtifact(artifact *Artifact) ([]byte, error) {
	if artifact == nil {
		return nil, fmt.Errorf("artifact is nil")
	}

	return json.MarshalIndent(
		artifact,
		"",
		"  ",
	)
}

func DeserializeArtifact(data []byte) (*Artifact, error) {
	var artifact Artifact

	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf(
			"deserializing artifact: %w",
			err,
		)
	}

	if artifact.Format != ArtifactFormat {
		return nil, fmt.Errorf(
			"unsupported artifact format %q",
			artifact.Format,
		)
	}

	if artifact.Version != ArtifactVersion {
		return nil, fmt.Errorf(
			"unsupported artifact version %q",
			artifact.Version,
		)
	}

	return &artifact, nil
}

func (a *Artifact) Decrypt(passphrase []byte) ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("artifact is nil")
	}

	aad := []byte(fmt.Sprintf(
		"%s:%s:%s:%s:%s",
		a.Format,
		a.Version,
		a.AccountID,
		a.Region,
		a.SnapshotHash,
	))

	payload, err := DecryptWithPassphrase(
		&a.Encrypted,
		passphrase,
		aad,
	)
	if err != nil {
		return nil, err
	}

	actualHash := HashBytes(payload)

	if actualHash != a.SnapshotHash {
		return nil, fmt.Errorf(
			"snapshot hash mismatch: expected %s, got %s",
			a.SnapshotHash,
			actualHash,
		)
	}

	return payload, nil
}
