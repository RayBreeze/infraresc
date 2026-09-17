package cli

import "github.com/spf13/cobra"

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Manage Infraresc recovery media",
}

var mediaCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Infraresc recovery media",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Creating recovery media...")
		return nil
	},
}

var mediaInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect Infraresc recovery media",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Inspecting recovery media...")
		return nil
	},
}

var mediaValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Infraresc recovery media",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Validating recovery media...")
		return nil
	},
}

func init() {
	mediaCmd.AddCommand(
		mediaCreateCmd,
		mediaInspectCmd,
		mediaValidateCmd,
	)

	rootCmd.AddCommand(mediaCmd)
}
