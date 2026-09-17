package state

import "time"

type Snapshot struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	AccountID string     `json:"account_id"`
	Region    string     `json:"region"`
	Resources []Resource `json:"resources"`
}
