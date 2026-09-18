package runtime

import (
	"context"
	"fmt"

	"infraresc/auth"
	"infraresc/aws"
	infraConfig "infraresc/config"
)

type Context struct {
	Identity *auth.Identity
	AWS      *aws.Client
	Config   infraConfig.Config
}

func Initialize(
	ctx context.Context,
	profile string,
) (*Context, error) {

	cfg := infraConfig.Load()

	if profile != "" {
		cfg.Profile = profile
	}

	client, err := aws.NewClient(
		ctx,
		cfg.Profile,
	)

	if err != nil {
		return nil, err
	}

	caller, err := client.Identity(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"AWS authentication required: %w",
			err,
		)
	}

	identity := &auth.Identity{
		Provider:      "AWS",
		AuthMethod:    "AWS Sign-In",
		AccountID:     caller.AccountID,
		PrincipalARN:  caller.PrincipalARN,
		UserID:        caller.UserID,
		Region:        caller.Region,
		Profile:       cfg.Profile,
		Authenticated: true,
	}

	return &Context{
		Identity: identity,
		AWS:      client,
		Config:   cfg,
	}, nil
}
