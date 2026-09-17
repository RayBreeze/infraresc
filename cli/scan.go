package cli

import "github.com/spf13/cobra"

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the cloud infrastructure",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Scanning for Cloud Infrastructure...")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
