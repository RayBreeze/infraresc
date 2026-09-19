package state

import "time"

type Snapshot struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`

	AccountID string `json:"account_id"`
	Region    string `json:"region"`

	Resources []Resource `json:"resources"`
	Edges     []Edge     `json:"edges"`

	Configs []ResourceConfig `json:"configs"`

	Warnings []SnapshotWarning `json:"warnings,omitempty"`
}

type ResourceConfig struct {
	ResourceID string `json:"resource_id"`
	ARN        string `json:"arn"`
	Type       string `json:"type"`
	Service    string `json:"service"`
	Region     string `json:"region"`

	Properties map[string]interface{} `json:"properties"`
}

type SnapshotWarning struct {
	ResourceID string `json:"resource_id"`
	Type       string `json:"type"`
	Message    string `json:"message"`
}
