package auth

import "context"

type LoginOptions struct {
	Profile string
}

type AuthProvider interface {
	Login(
		ctx context.Context,
		opts LoginOptions,
	) (*Identity, error)

	Logout(
		ctx context.Context,
		opts LoginOptions,
	) error

	Status(
		ctx context.Context,
		opts LoginOptions,
	) (*Identity, error)
}
