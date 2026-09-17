package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "infraresc",
	Short: "Infraresc is a portable cloud infrastructure recovery tool",
	Long:  `Infraresc is a portable cloud infrastructure recovery tool for capturing, verifying and recovering cloud infrastructure using physical media.`,
}

func Execute() error {
	return rootCmd.Execute()
}
