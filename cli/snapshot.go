package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	infraAWS "infraresc/aws"
	infraRuntime "infraresc/runtime"

	"github.com/spf13/cobra"
)

var snapshotProfile string
var snapshotOutput string

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Capture AWS infrastructure configuration",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		ctx := cmd.Context()

		rt, err := infraRuntime.Initialize(
			ctx,
			snapshotProfile,
		)

		if err != nil {
			return fmt.Errorf(
				"%w\nRun 'infraresc auth login' first",
				err,
			)
		}

		fmt.Println("InfraResc Snapshot")
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

		fmt.Println("Discovering resources...")

		graph, err := collector.Collect(ctx)

		if err != nil {
			return err
		}

		fmt.Printf(
			"  Resources:     %d\n",
			len(graph.Resources),
		)

		fmt.Printf(
			"  Relationships: %d\n",
			len(graph.Edges),
		)

		fmt.Println()
		fmt.Println("Capturing configuration...")

		snapshotCollector := infraAWS.NewSnapshotCollector(
			rt.AWS,
		)

		snapshot, err := snapshotCollector.Collect(
			ctx,
			rt.Identity.AccountID,
			rt.Identity.Region,
			graph.Resources,
			graph.Edges,
		)

		if err != nil {
			return fmt.Errorf(
				"creating snapshot: %w",
				err,
			)
		}

		data, err := json.MarshalIndent(
			snapshot,
			"",
			"  ",
		)

		if err != nil {
			return fmt.Errorf(
				"serializing snapshot: %w",
				err,
			)
		}

		output := snapshotOutput

		if output == "" {
			output = fmt.Sprintf(
				"infraresc-snapshot-%s.json",
				time.Now().UTC().Format(
					"20060102-150405",
				),
			)
		}

		if err := os.WriteFile(
			output,
			data,
			0644,
		); err != nil {
			return fmt.Errorf(
				"writing snapshot: %w",
				err,
			)
		}

		fmt.Println()
		fmt.Println("Snapshot complete.")
		fmt.Println()

		fmt.Printf(
			"Resources:       %d\n",
			len(snapshot.Resources),
		)

		fmt.Printf(
			"Relationships:   %d\n",
			len(snapshot.Edges),
		)

		fmt.Printf(
			"Configurations:  %d\n",
			len(snapshot.Configs),
		)

		fmt.Printf(
			"Warnings:        %d\n",
			len(snapshot.Warnings),
		)

		fmt.Println()
		fmt.Printf(
			"Output: %s\n",
			output,
		)

		return nil
	},
}

func init() {

	snapshotCmd.Flags().StringVarP(
		&snapshotProfile,
		"profile",
		"p",
		"",
		"AWS profile to use",
	)

	snapshotCmd.Flags().StringVarP(
		&snapshotOutput,
		"output",
		"o",
		"",
		"Snapshot output file",
	)

	rootCmd.AddCommand(
		snapshotCmd,
	)
}
