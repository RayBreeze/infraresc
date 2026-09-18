package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	oidcTypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/pkg/browser"
)

type DeviceAuthOptions struct {
	StartURL  string
	SSORegion string
}

type AuthResult struct {
	AccessToken string
	ExpiresIn   int32
}

// LoginWithDeviceCode runs the OAuth 2.0 device authorization grant flow.
func LoginWithDeviceCode(ctx context.Context, opts DeviceAuthOptions) (*AuthResult, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.SSORegion))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	oidcClient := ssooidc.NewFromConfig(cfg)

	// 1. Register temporary public client
	reg, err := oidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: awsSDK.String("infrakey-cli"),
		ClientType: awsSDK.String("public"),
	})
	if err != nil {
		return nil, fmt.Errorf("registering OIDC client failed: %w", err)
	}

	// 2. Start device authorization
	auth, err := oidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     reg.ClientId,
		ClientSecret: reg.ClientSecret,
		StartUrl:     awsSDK.String(opts.StartURL),
	})
	if err != nil {
		return nil, fmt.Errorf("initiating device auth failed: %w", err)
	}

	// 3. Prompt user and open verification URL
	verificationURI := awsSDK.ToString(auth.VerificationUriComplete)
	userCode := awsSDK.ToString(auth.UserCode)

	fmt.Printf("\nUser Code: %s\n", userCode)
	fmt.Printf("Opening browser to authorize: %s\n", verificationURI)
	_ = browser.OpenURL(verificationURI)

	// 4. Poll for access token
	interval := time.Duration(auth.Interval) * time.Second
	if interval == 0 {
		interval = 3 * time.Second
	}

	fmt.Println("Waiting for browser confirmation...")

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			tokenRes, err := oidcClient.CreateToken(ctx, &ssooidc.CreateTokenInput{
				ClientId:     reg.ClientId,
				ClientSecret: reg.ClientSecret,
				GrantType:    awsSDK.String("urn:ietf:params:oauth:grant-type:device_code"),
				DeviceCode:   auth.DeviceCode,
			})

			if err != nil {
				var authPending *oidcTypes.AuthorizationPendingException
				var slowDown *oidcTypes.SlowDownException

				if errors.As(err, &authPending) {
					time.Sleep(interval)
					continue
				}
				if errors.As(err, &slowDown) {
					interval += 2 * time.Second
					time.Sleep(interval)
					continue
				}
				return nil, fmt.Errorf("device authentication failed: %w", err)
			}

			return &AuthResult{
				AccessToken: awsSDK.ToString(tokenRes.AccessToken),
				ExpiresIn:   tokenRes.ExpiresIn,
			}, nil
		}
	}
}

// GetRoleCredentials exchanges the SSO access token for standard STS credentials.
func GetRoleCredentials(ctx context.Context, region, accountID, roleName, accessToken string) (*sso.GetRoleCredentialsOutput, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	ssoClient := sso.NewFromConfig(cfg)
	return ssoClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: awsSDK.String(accessToken),
		AccountId:   awsSDK.String(accountID),
		RoleName:    awsSDK.String(roleName),
	})
}
