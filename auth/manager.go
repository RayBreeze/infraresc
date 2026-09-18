package auth

import "context"

type Manager struct {
	provider AuthProvider
}

func NewManager(
	provider AuthProvider,
) *Manager {

	return &Manager{
		provider: provider,
	}
}

func (m *Manager) Login(
	ctx context.Context,
	opts LoginOptions,
) (*Identity, error) {

	return m.provider.Login(
		ctx,
		opts,
	)
}

func (m *Manager) Logout(
	ctx context.Context,
	opts LoginOptions,
) error {

	return m.provider.Logout(
		ctx,
		opts,
	)
}

func (m *Manager) Status(
	ctx context.Context,
	opts LoginOptions,
) (*Identity, error) {

	return m.provider.Status(
		ctx,
		opts,
	)
}
