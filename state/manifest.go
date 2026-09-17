package state

type Manifest struct {
	Version   string            `json:"version"`
	Snapshot  string            `json:"snapshot"`
	Algorithm string            `json:"algorithm"`
	Files     map[string]string `json:"files"`
	RootHash  string            `json:"root_hash"`
}
