package state

type RecoveryAction string

const (
	RecoveryMissing RecoveryAction = "missing"
	RecoveryChanged RecoveryAction = "changed"
)

type RecoveryItem struct {
	ResourceID   string         `json:"resource_id"`
	ResourceType string         `json:"resource_type"`
	Action       RecoveryAction `json:"action"`
}

type RecoveryResult struct {
	Attempted int `json:"attempted"`
	Recovered int `json:"recovered"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`

	Items  []RecoveryItem `json:"items"`
	Errors []string       `json:"errors,omitempty"`

	RecoveredIDs []string `json:"recovered_ids,omitempty"`
}
