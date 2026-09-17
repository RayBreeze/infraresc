package state

type Resource struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Name          string         `json:"name,omitempty"`
	Region        string         `json:"region"`
	Configuration map[string]any `json:"configuration"`
	Dependencies  []string       `json:"dependencies,omitempty"`
}
