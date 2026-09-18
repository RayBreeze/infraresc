package cli

import (
	"context"
	"encoding/json"
	"fmt"
	infraAWS "infraresc/aws"

	"github.com/spf13/cobra"

	infraRuntime "infraresc/runtime"
)

var scanProfile string

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover AWS infrastructure",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		ctx := context.Background()

		rt, err := infraRuntime.Initialize(
			ctx,
			scanProfile,
		)

		if err != nil {
			return fmt.Errorf(
				"%w\nRun 'infraresc auth login' first",
				err,
			)
		}

		fmt.Println("InfraResc Discovery")
		fmt.Println()

		fmt.Printf(
			"Account:  %s\n",
			rt.Identity.AccountID,
		)

		fmt.Printf(
			"Region:   %s\n",
			rt.Identity.Region,
		)

		fmt.Printf(
			"Profile:  %s\n",
			rt.Identity.Profile,
		)

		fmt.Println()

		collector := infraAWS.NewCollector(rt.AWS)

		result, err := collector.Collect(ctx)

		if err != nil {
			return err
		}

		fmt.Printf(
			"Discovered %d resources\n",
			len(result.Resources),
		)

		fmt.Printf(
			"Discovered %d relationships\n",
			len(result.Edges),
		)

		fmt.Println()

		data, err := json.MarshalIndent(
			result,
			"",
			"  ",
		)

		if err != nil {
			return fmt.Errorf(
				"serializing infrastructure graph: %w",
				err,
			)
		}

		fmt.Println(string(data))

		return nil
	},
}

func init() {

	scanCmd.Flags().StringVarP(
		&scanProfile,
		"profile",
		"p",
		"",
		"AWS profile to use",
	)

	rootCmd.AddCommand(
		scanCmd,
	)
}
