package auth

type Identity struct {
	Provider      string
	AuthMethod    string
	AccountID     string
	PrincipalARN  string
	UserID        string
	Region        string
	Profile       string
	Authenticated bool
}
