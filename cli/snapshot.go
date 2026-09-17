package cli

import "github.com/spf13/cobra"

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Capture a snapshot of the cloud infrastructure",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Creating infrastructure snapshot...")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
