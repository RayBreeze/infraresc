package cli

import "github.com/spf13/cobra"

var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover cloud infrastructure",
}

var recoverPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate a recovery plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Generating recovery plan...")
		return nil
	},
}

var recoverExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute a recovery plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("Executing recovery...")
		return nil
	},
}

func init() {
	recoverCmd.AddCommand(
		recoverPlanCmd,
		recoverExecuteCmd,
	)

	rootCmd.AddCommand(recoverCmd)
}
