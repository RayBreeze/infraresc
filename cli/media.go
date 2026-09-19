package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	infraMedia "infraresc/media"

	"github.com/spf13/cobra"
)

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Manage InfraResc physical backup media",
}

var mediaStoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Store an encrypted snapshot on external media",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		fmt.Println("InfraResc Media Store")
		fmt.Println()

		snapshot, err := chooseSnapshot()
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Select external media")
		fmt.Println()

		mediaRoot, err := chooseMedia()
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Selected media:")
		fmt.Printf("  %s\n", mediaRoot)
		fmt.Println()

		isInfraResc := infraMedia.IsInfraRescMedia(
			mediaRoot,
		)

		if !isInfraResc {

			fmt.Println("WARNING")
			fmt.Println()
			fmt.Println(
				"This external media will be formatted and prepared",
			)
			fmt.Println(
				"for InfraResc physical storage.",
			)
			fmt.Println()
			fmt.Println(
				"ALL EXISTING DATA ON THIS MEDIA WILL BE DESTROYED.",
			)
			fmt.Println()
			fmt.Printf(
				"Selected media: %s\n",
				mediaRoot,
			)
			fmt.Println()
			fmt.Print("Continue? [y/N]: ")

			answer := readLine()

			if !strings.EqualFold(
				answer,
				"y",
			) {
				fmt.Println()
				fmt.Println("Operation cancelled.")
				return nil
			}

			fmt.Println()
			fmt.Println("Preparing external media...")

			if err := infraMedia.FormatExternalMedia(
				mediaRoot,
			); err != nil {
				return fmt.Errorf(
					"preparing external media: %w",
					err,
				)
			}

			fmt.Println("Media formatted successfully.")
			fmt.Println()
		}

		fmt.Println("Storing snapshot...")

		entry, err := infraMedia.StoreSnapshot(
			mediaRoot,
			snapshot,
		)

		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Snapshot stored successfully.")
		fmt.Println()
		fmt.Printf(
			"  File:   %s\n",
			entry.Filename,
		)
		fmt.Printf(
			"  Size:   %d bytes\n",
			entry.Size,
		)
		fmt.Printf(
			"  SHA256: %s\n",
			entry.SHA256,
		)
		fmt.Printf(
			"  Media:  %s\n",
			mediaRoot,
		)

		return nil
	},
}

func chooseSnapshot() (string, error) {

	files, err := filepath.Glob("*.irs")
	if err != nil {
		return "", fmt.Errorf(
			"finding snapshots: %w",
			err,
		)
	}

	if len(files) == 0 {
		return "", fmt.Errorf(
			"no .irs snapshots found in the current directory",
		)
	}

	fmt.Println("Available snapshots:")
	fmt.Println()

	for i, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		fmt.Printf(
			"%d. %s (%d bytes)\n",
			i+1,
			file,
			info.Size(),
		)
	}

	fmt.Println()
	fmt.Print("Select snapshot: ")

	choice := readLine()

	index, err := strconv.Atoi(choice)
	if err != nil {
		return "", fmt.Errorf(
			"invalid snapshot selection",
		)
	}

	if index < 1 || index > len(files) {
		return "", fmt.Errorf(
			"snapshot selection out of range",
		)
	}

	return files[index-1], nil
}

func chooseMedia() (string, error) {

	media, err := infraMedia.DiscoverExternalMedia()
	if err != nil {
		return "", err
	}

	if len(media) == 0 {
		return "", fmt.Errorf(
			"no external media detected",
		)
	}

	for i, item := range media {
		fmt.Printf(
			"%d. %s\n",
			i+1,
			item,
		)
	}

	fmt.Println()
	fmt.Print("Select media: ")

	choice := readLine()

	index, err := strconv.Atoi(choice)
	if err != nil {
		return "", fmt.Errorf(
			"invalid media selection",
		)
	}

	if index < 1 || index > len(media) {
		return "", fmt.Errorf(
			"media selection out of range",
		)
	}

	return media[index-1], nil
}

func readLine() string {

	reader := bufio.NewReader(
		os.Stdin,
	)

	value, _ := reader.ReadString(
		'\n',
	)

	return strings.TrimSpace(value)
}

func init() {

	mediaCmd.AddCommand(
		mediaStoreCmd,
	)

	rootCmd.AddCommand(
		mediaCmd,
	)
}
