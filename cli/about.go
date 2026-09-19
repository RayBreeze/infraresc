package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "About InfraResc",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		fmt.Println("InfraResc")
		fmt.Println("Version: 0.1.0")
		fmt.Println("License: Apache-2.0")
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(aboutCmd)
}
