package media

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func StoreSnapshot(
	mediaRoot string,
	snapshotPath string,
) (*SnapshotEntry, error) {

	info, err := os.Stat(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf(
			"reading snapshot: %w",
			err,
		)
	}

	if info.IsDir() {
		return nil, fmt.Errorf(
			"snapshot path is a directory",
		)
	}

	if filepath.Ext(snapshotPath) != ".irs" {
		return nil, fmt.Errorf(
			"selected file is not an InfraResc .irs artifact",
		)
	}

	hash, err := SHA256File(snapshotPath)
	if err != nil {
		return nil, err
	}

	manifest, err := Initialize(mediaRoot)
	if err != nil {
		return nil, err
	}

	destination := filepath.Join(
		SnapshotPath(mediaRoot),
		filepath.Base(snapshotPath),
	)

	// Don't overwrite an existing snapshot.
	if _, err := os.Stat(destination); err == nil {
		return nil, fmt.Errorf(
			"snapshot already exists on this media: %s",
			filepath.Base(snapshotPath),
		)
	}

	source, err := os.Open(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf(
			"opening snapshot: %w",
			err,
		)
	}
	defer source.Close()

	target, err := os.OpenFile(
		destination,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0600,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"creating snapshot on media: %w",
			err,
		)
	}
	defer target.Close()

	if _, err := io.Copy(
		target,
		source,
	); err != nil {
		return nil, fmt.Errorf(
			"copying snapshot to media: %w",
			err,
		)
	}

	entry := SnapshotEntry{
		ID:       hash[:16],
		Filename: filepath.Base(snapshotPath),
		Size:     info.Size(),
		SHA256:   hash,
		StoredAt: time.Now().UTC(),
	}

	manifest.Snapshots = append(
		manifest.Snapshots,
		entry,
	)

	if err := SaveManifest(
		mediaRoot,
		manifest,
	); err != nil {
		return nil, err
	}

	return &entry, nil
}
