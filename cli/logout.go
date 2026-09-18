package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"infraresc/auth"
	infraConfig "infraresc/config"
)

var logoutProfile string

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from AWS",
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := context.Background()

		profile := infraConfig.ResolveProfile(
			logoutProfile,
		)

		manager := auth.NewManager(
			auth.NewAWSLoginProvider(),
		)

		if err := manager.Logout(
			ctx,
			auth.LoginOptions{
				Profile: profile,
			},
		); err != nil {
			return err
		}

		fmt.Printf(
			"✓ Logged out from AWS profile %q\n",
			profile,
		)

		return nil
	},
}

func init() {

	logoutCmd.Flags().StringVarP(
		&logoutProfile,
		"profile",
		"p",
		"",
		"AWS profile to logout",
	)

	authCmd.AddCommand(
		logoutCmd,
	)
}
