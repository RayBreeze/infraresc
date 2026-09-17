package cli

import "github.com/spf13/cobra"

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify snapshots and recovery media",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Verifying...")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
