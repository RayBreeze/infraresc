package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	infraRecover "infraresc/recover"
	infraRuntime "infraresc/runtime"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover missing or changed cloud infrastructure",

	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := context.Background()

		fmt.Println("InfraResc Recovery")
		fmt.Println()

		// --------------------------------------------------------
		// SELECT SNAPSHOT FROM PHYSICAL MEDIA
		// --------------------------------------------------------

		snapshotPath, err := chooseDiffSnapshot()
		if err != nil {
			return err
		}

		fmt.Println()

		fmt.Printf(
			"Selected snapshot: %s\n",
			snapshotPath,
		)

		// --------------------------------------------------------
		// DECRYPT SNAPSHOT
		// --------------------------------------------------------

		fmt.Println()
		fmt.Print("Encryption password: ")

		passwordBytes, err := term.ReadPassword(
			int(os.Stdin.Fd()),
		)

		fmt.Println()

		if err != nil {
			return fmt.Errorf(
				"reading password: %w",
				err,
			)
		}

		password := strings.TrimSpace(
			string(passwordBytes),
		)

		passwordBytes = nil

		if password == "" {
			return fmt.Errorf(
				"password cannot be empty",
			)
		}

		fmt.Println()
		fmt.Println("Decrypting snapshot...")

		snapshot, err := loadDiffSnapshot(
			snapshotPath,
			password,
		)

		password = ""

		if err != nil {
			return err
		}

		fmt.Println("Authentication: OK")

		// --------------------------------------------------------
		// VALIDATE SNAPSHOT
		// --------------------------------------------------------

		if err := validateDiffSnapshot(snapshot); err != nil {
			return fmt.Errorf(
				"snapshot validation failed: %w",
				err,
			)
		}

		fmt.Println("Snapshot validation: OK")

		// --------------------------------------------------------
		// INITIALIZE AWS
		// --------------------------------------------------------

		fmt.Println()
		fmt.Println(
			"Connecting to live AWS infrastructure...",
		)

		rt, err := infraRuntime.Initialize(
			ctx,
			"",
		)

		if err != nil {
			return fmt.Errorf(
				"initializing AWS runtime: %w",
				err,
			)
		}

		fmt.Printf(
			"Account: %s\n",
			rt.Identity.AccountID,
		)

		fmt.Printf(
			"Region:  %s\n",
			rt.Identity.Region,
		)

		// --------------------------------------------------------
		// SAFETY CHECK
		// --------------------------------------------------------

		if snapshot.AccountID != rt.Identity.AccountID {
			return fmt.Errorf(
				"snapshot account %s does not match live AWS account %s",
				snapshot.AccountID,
				rt.Identity.AccountID,
			)
		}

		if snapshot.Region != rt.Identity.Region {
			return fmt.Errorf(
				"snapshot region %s does not match live AWS region %s",
				snapshot.Region,
				rt.Identity.Region,
			)
		}

		// --------------------------------------------------------
		// RECOVERY ENGINE
		// --------------------------------------------------------

		fmt.Println()
		fmt.Println(
			"Verifying live infrastructure...",
		)

		engine := infraRecover.New(
			rt.AWS,
		)

		result, err := engine.Run(
			ctx,
			snapshot,
		)

		// --------------------------------------------------------
		// RECOVERY SUMMARY
		// --------------------------------------------------------

		if result != nil {

			fmt.Println()
			fmt.Println("Recovery Summary")
			fmt.Println("----------------------------------------")

			fmt.Printf(
				"Attempted: %d\n",
				result.Attempted,
			)

			fmt.Printf(
				"Recovered: %d\n",
				result.Recovered,
			)

			fmt.Printf(
				"Skipped:   %d\n",
				result.Skipped,
			)

			fmt.Printf(
				"Failed:    %d\n",
				result.Failed,
			)

			if len(result.Errors) > 0 {
				fmt.Println()

				for _, recoveryErr := range result.Errors {
					fmt.Printf(
						"  ERROR: %s\n",
						recoveryErr,
					)
				}
			}
		}

		// --------------------------------------------------------
		// ENGINE ERROR
		// --------------------------------------------------------

		/*
		 * An error returned by the recovery engine is still a
		 * genuine infrastructure/recovery error.
		 *
		 * A non-zero Failed count by itself is NOT an engine
		 * failure anymore. It represents partial recovery.
		 */
		if err != nil {
			return fmt.Errorf(
				"recovery failed: %w",
				err,
			)
		}

		// --------------------------------------------------------
		// FINAL STATUS
		// --------------------------------------------------------

		fmt.Println()

		switch {
		case result == nil:
			return fmt.Errorf(
				"recovery completed without a result",
			)

		case result.Failed == 0 && result.Recovered > 0:
			fmt.Println(
				"Recovery completed successfully.",
			)

		case result.Failed > 0 && result.Recovered > 0:
			fmt.Println(
				"Recovery completed with partial success.",
			)

		case result.Failed > 0 && result.Recovered == 0:
			fmt.Println(
				"Recovery completed with no resources recovered.",
			)

		default:
			fmt.Println(
				"No recovery was required.",
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(
		recoverCmd,
	)
}
