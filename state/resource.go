package state

type Resource struct {
	ID      string `json:"id"`
	ARN     string `json:"arn"`
	Type    string `json:"type"`
	Service string `json:"service"`
	Region  string `json:"region"`
}

type Edge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

type Infrastructure struct {
	Resources []Resource `json:"resources"`
	Edges     []Edge     `json:"edges"`
}
