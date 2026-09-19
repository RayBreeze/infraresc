package media

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	MediaDirectory = "InfraResc"
	SnapshotDir    = "snapshots"
	ManifestFile   = "manifest.json"
	MediaVersion   = "1"
)

type Manifest struct {
	Format    string          `json:"format"`
	Version   string          `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	Snapshots []SnapshotEntry `json:"snapshots"`
}

type SnapshotEntry struct {
	ID       string    `json:"id"`
	Filename string    `json:"filename"`
	Size     int64     `json:"size"`
	SHA256   string    `json:"sha256"`
	StoredAt time.Time `json:"stored_at"`
}

func RepositoryPath(mediaRoot string) string {
	return filepath.Join(
		mediaRoot,
		MediaDirectory,
	)
}

func SnapshotPath(mediaRoot string) string {
	return filepath.Join(
		RepositoryPath(mediaRoot),
		SnapshotDir,
	)
}

func ManifestPath(mediaRoot string) string {
	return filepath.Join(
		RepositoryPath(mediaRoot),
		ManifestFile,
	)
}

func IsInfraRescMedia(mediaRoot string) bool {
	_, err := os.Stat(ManifestPath(mediaRoot))
	return err == nil
}

func LoadManifest(mediaRoot string) (*Manifest, error) {
	path := ManifestPath(mediaRoot)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"reading InfraResc media manifest: %w",
			err,
		)
	}

	var manifest Manifest

	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf(
			"invalid InfraResc media manifest: %w",
			err,
		)
	}

	return &manifest, nil
}

func SaveManifest(
	mediaRoot string,
	manifest *Manifest,
) error {

	repository := RepositoryPath(mediaRoot)

	if err := os.MkdirAll(
		repository,
		0755,
	); err != nil {
		return fmt.Errorf(
			"creating InfraResc media directory: %w",
			err,
		)
	}

	data, err := json.MarshalIndent(
		manifest,
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf(
			"serializing media manifest: %w",
			err,
		)
	}

	if err := os.WriteFile(
		ManifestPath(mediaRoot),
		data,
		0644,
	); err != nil {
		return fmt.Errorf(
			"writing media manifest: %w",
			err,
		)
	}

	return nil
}

func Initialize(
	mediaRoot string,
) (*Manifest, error) {

	if IsInfraRescMedia(mediaRoot) {
		return LoadManifest(mediaRoot)
	}

	manifest := &Manifest{
		Format:    "infraresc-media",
		Version:   MediaVersion,
		CreatedAt: time.Now().UTC(),
		Snapshots: make([]SnapshotEntry, 0),
	}

	if err := SaveManifest(
		mediaRoot,
		manifest,
	); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(
		SnapshotPath(mediaRoot),
		0755,
	); err != nil {
		return nil, fmt.Errorf(
			"creating snapshot directory: %w",
			err,
		)
	}

	return manifest, nil
}
