package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"infraresc/auth"
	infraConfig "infraresc/config"
)

var authProfile string

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage AWS authentication",
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show AWS authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := context.Background()

		profile := infraConfig.ResolveProfile(
			authProfile,
		)

		manager := auth.NewManager(
			auth.NewAWSLoginProvider(),
		)

		identity, err := manager.Status(
			ctx,
			auth.LoginOptions{
				Profile: profile,
			},
		)

		if err != nil {

			fmt.Println("AWS Authentication")
			fmt.Println()
			fmt.Println("Status: NOT AUTHENTICATED")
			fmt.Println()
			fmt.Printf(
				"Run: infraresc auth login --profile %s\n",
				profile,
			)

			return nil
		}

		fmt.Println("AWS Authentication")
		fmt.Println()
		fmt.Println("Status:      Authenticated")
		fmt.Printf(
			"Provider:    %s\n",
			identity.Provider,
		)
		fmt.Printf(
			"Method:      %s\n",
			identity.AuthMethod,
		)
		fmt.Printf(
			"Account:     %s\n",
			identity.AccountID,
		)
		fmt.Printf(
			"Principal:   %s\n",
			identity.PrincipalARN,
		)
		fmt.Printf(
			"Region:      %s\n",
			identity.Region,
		)
		fmt.Printf(
			"Profile:     %s\n",
			identity.Profile,
		)

		return nil
	},
}

func init() {

	authStatusCmd.Flags().StringVarP(
		&authProfile,
		"profile",
		"p",
		"",
		"AWS profile to inspect",
	)

	authCmd.AddCommand(
		authStatusCmd,
	)

	rootCmd.AddCommand(
		authCmd,
	)
}
