package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	infraAWS "infraresc/aws"
	infraCrypto "infraresc/crypto"
	infraGraph "infraresc/graph"
	infraRuntime "infraresc/runtime"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var snapshotProfile string
var snapshotOutput string

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Capture, graph, and encrypt AWS infrastructure",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		ctx := cmd.Context()

		// ------------------------------------------------------------
		// AWS INITIALIZATION
		// ------------------------------------------------------------

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

		// ------------------------------------------------------------
		// DISCOVERY
		// ------------------------------------------------------------

		collector := infraAWS.NewCollector(
			rt.AWS,
		)

		fmt.Println(
			"Discovering resources and relationships...",
		)

		infrastructure, err := collector.Collect(
			ctx,
		)

		if err != nil {
			return err
		}

		fmt.Printf(
			"  Resources:     %d\n",
			len(infrastructure.Resources),
		)

		fmt.Printf(
			"  Relationships: %d\n",
			len(infrastructure.Edges),
		)

		// ------------------------------------------------------------
		// GRAPH
		// ------------------------------------------------------------

		fmt.Println()
		fmt.Println(
			"Building dependency graph...",
		)

		dependencyGraph := infraGraph.BuildWithEdges(
			infrastructure.Resources,
			infrastructure.Edges,
		)

		order, err := dependencyGraph.ResolveOrder()

		if err != nil {
			return fmt.Errorf(
				"resolving dependency graph: %w",
				err,
			)
		}

		fmt.Printf(
			"  Graph nodes:    %d\n",
			len(dependencyGraph.Nodes),
		)

		fmt.Printf(
			"  Recovery order: %d\n",
			len(order),
		)

		// ------------------------------------------------------------
		// CONFIGURATION SNAPSHOT
		// ------------------------------------------------------------

		fmt.Println()
		fmt.Println(
			"Capturing configuration...",
		)

		snapshotCollector := infraAWS.NewSnapshotCollector(
			rt.AWS,
		)

		snapshot, err := snapshotCollector.Collect(
			ctx,
			rt.Identity.AccountID,
			rt.Identity.Region,
			infrastructure.Resources,
			infrastructure.Edges,
		)

		if err != nil {
			return fmt.Errorf(
				"creating snapshot: %w",
				err,
			)
		}

		// Add dependency graph to snapshot.
		snapshot.Graph = dependencyGraph.Snapshot()

		// ------------------------------------------------------------
		// PASSWORD
		// ------------------------------------------------------------

		fmt.Println()
		fmt.Println("Snapshot encryption")
		fmt.Println()

		password, err := promptPassword(
			"Encryption password: ",
		)

		if err != nil {
			return err
		}

		confirmation, err := promptPassword(
			"Confirm password:    ",
		)

		if err != nil {
			return err
		}

		if password != confirmation {
			return fmt.Errorf(
				"passwords do not match",
			)
		}

		// ------------------------------------------------------------
		// SERIALIZATION
		// ------------------------------------------------------------

		fmt.Println()
		fmt.Println(
			"Serializing snapshot...",
		)

		plaintext, err := json.Marshal(
			snapshot,
		)

		if err != nil {
			return fmt.Errorf(
				"serializing snapshot: %w",
				err,
			)
		}

		// ------------------------------------------------------------
		// ENCRYPTION
		// ------------------------------------------------------------

		fmt.Println(
			"Encrypting snapshot...",
		)

		artifact, err := infraCrypto.Seal(
			plaintext,
			password,
		)

		// Clear local password variables after use.
		password = ""
		confirmation = ""

		if err != nil {
			return fmt.Errorf(
				"encrypting snapshot: %w",
				err,
			)
		}

		// ------------------------------------------------------------
		// OUTPUT
		// ------------------------------------------------------------

		output := snapshotOutput

		if output == "" {

			output = fmt.Sprintf(
				"infraresc-snapshot-%s.irs",
				time.Now().UTC().Format(
					"20060102-150405",
				),
			)
		}

		if err := os.WriteFile(
			output,
			artifact,
			0600,
		); err != nil {
			return fmt.Errorf(
				"writing protected snapshot: %w",
				err,
			)
		}

		// ------------------------------------------------------------
		// RESULT
		// ------------------------------------------------------------

		fmt.Println()
		fmt.Println(
			"Protected snapshot complete.",
		)

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
			"Graph nodes:     %d\n",
			len(snapshot.Graph.Nodes),
		)

		fmt.Printf(
			"Configurations:  %d\n",
			len(snapshot.Configs),
		)

		fmt.Printf(
			"Warnings:        %d\n",
			len(snapshot.Warnings),
		)

		fmt.Printf(
			"Output:          %s\n",
			output,
		)

		return nil
	},
}

// promptPassword reads a password without echoing it to the terminal.
func promptPassword(
	prompt string,
) (string, error) {

	fmt.Print(prompt)

	password, err := term.ReadPassword(
		int(os.Stdin.Fd()),
	)

	fmt.Println()

	if err != nil {
		return "", fmt.Errorf(
			"reading password: %w",
			err,
		)
	}

	value := strings.TrimSpace(
		string(password),
	)

	if value == "" {
		return "", fmt.Errorf(
			"password cannot be empty",
		)
	}

	return value, nil
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
		"Protected snapshot output file",
	)

	rootCmd.AddCommand(
		snapshotCmd,
	)
}
