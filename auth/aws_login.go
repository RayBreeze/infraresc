package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"infraresc/aws"
)

type AWSLoginProvider struct {
	CLIPath string
}

func NewAWSLoginProvider() *AWSLoginProvider {
	return &AWSLoginProvider{
		CLIPath: "aws",
	}
}

func (p *AWSLoginProvider) Login(
	ctx context.Context,
	opts LoginOptions,
) (*Identity, error) {

	profile := opts.Profile

	if profile == "" {
		profile = "infraresc"
	}

	fmt.Printf(
		"Starting AWS browser login using profile %q...\n",
		profile,
	)

	fmt.Println()
	fmt.Println("Opening browser...")

	cmd := exec.CommandContext(
		ctx,
		p.CLIPath,
		"login",
		"--profile",
		profile,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf(
			"AWS login failed: %w\n%s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	fmt.Println()
	fmt.Println("AWS browser authentication completed.")

	return p.Status(
		ctx,
		opts,
	)
}

func (p *AWSLoginProvider) Logout(
	ctx context.Context,
	opts LoginOptions,
) error {

	profile := opts.Profile

	if profile == "" {
		profile = "infraresc"
	}

	cmd := exec.CommandContext(
		ctx,
		p.CLIPath,
		"logout",
		"--profile",
		profile,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf(
			"AWS logout failed: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

func (p *AWSLoginProvider) Status(
	ctx context.Context,
	opts LoginOptions,
) (*Identity, error) {

	profile := opts.Profile

	if profile == "" {
		profile = "infraresc"
	}

	cmd := exec.CommandContext(
		ctx,
		p.CLIPath,
		"sts",
		"get-caller-identity",
		"--profile",
		profile,
		"--output",
		"json",
	)

	output, err := cmd.Output()

	if err != nil {
		return nil, fmt.Errorf(
			"AWS authentication is not active: %w",
			err,
		)
	}

	var result struct {
		UserID  string `json:"UserId"`
		Account string `json:"Account"`
		ARN     string `json:"Arn"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf(
			"failed to parse AWS identity: %w",
			err,
		)
	}

	region, err := aws.ProfileRegion(
		ctx,
		p.CLIPath,
		profile,
	)

	if err != nil {
		return nil, err
	}

	return &Identity{
		Provider:      "AWS",
		AuthMethod:    "AWS Sign-In",
		AccountID:     result.Account,
		PrincipalARN:  result.ARN,
		UserID:        result.UserID,
		Region:        region,
		Profile:       profile,
		Authenticated: true,
	}, nil
}
