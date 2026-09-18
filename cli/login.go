package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"infraresc/auth"
	infraConfig "infraresc/config"
)

var loginProfile string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to AWS through browser authentication",
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := context.Background()

		profile := infraConfig.ResolveProfile(
			loginProfile,
		)

		manager := auth.NewManager(
			auth.NewAWSLoginProvider(),
		)

		identity, err := manager.Login(
			ctx,
			auth.LoginOptions{
				Profile: profile,
			},
		)

		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("InfraResc Authentication")
		fmt.Println()
		fmt.Println("✓ Authentication successful")
		fmt.Println()
		fmt.Printf(
			"Provider:   %s\n",
			identity.Provider,
		)
		fmt.Printf(
			"Method:     %s\n",
			identity.AuthMethod,
		)
		fmt.Printf(
			"Account:    %s\n",
			identity.AccountID,
		)
		fmt.Printf(
			"Principal:  %s\n",
			identity.PrincipalARN,
		)
		fmt.Printf(
			"Region:     %s\n",
			identity.Region,
		)
		fmt.Printf(
			"Profile:    %s\n",
			identity.Profile,
		)

		return nil
	},
}

func init() {

	loginCmd.Flags().StringVarP(
		&loginProfile,
		"profile",
		"p",
		"",
		"AWS profile to authenticate",
	)

	authCmd.AddCommand(
		loginCmd,
	)
}
