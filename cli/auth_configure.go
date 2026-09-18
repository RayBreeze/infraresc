package cli

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"infraresc/auth"
)

var authConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure AWS authentication",
	RunE: func(cmd *cobra.Command, args []string) error {

		reader := bufio.NewReader(os.Stdin)

		profile, err := auth.InteractiveConfigure(
			reader,
		)

		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf(
			"✓ AWS profile %q configured successfully.\n",
			profile.Name,
		)

		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Printf(
			"  Profile: %s\n",
			profile.Name,
		)
		fmt.Printf(
			"  Region:  %s\n",
			profile.Region,
		)

		fmt.Println()
		fmt.Println("Next step:")
		fmt.Printf(
			"  infraresc auth login --profile %s\n",
			profile.Name,
		)

		return nil
	},
}

func init() {
	authCmd.AddCommand(
		authConfigureCmd,
	)
}
