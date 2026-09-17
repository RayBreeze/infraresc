package cli

import "github.com/spf13/cobra"

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare infrastructure states",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Comparing infrastructure states...")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
