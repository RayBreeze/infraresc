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

// ------------------------------------------------------------
// STORE
// ------------------------------------------------------------

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

			if !strings.EqualFold(answer, "y") {
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
		fmt.Printf("  File:   %s\n", entry.Filename)
		fmt.Printf("  Size:   %d bytes\n", entry.Size)
		fmt.Printf("  SHA256: %s\n", entry.SHA256)
		fmt.Printf("  Media:  %s\n", mediaRoot)

		return nil
	},
}

// ------------------------------------------------------------
// LIST
// ------------------------------------------------------------

var mediaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List snapshots stored on external media",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		fmt.Println("InfraResc Media List")
		fmt.Println()

		mediaRoot, err := chooseInfraRescMedia()
		if err != nil {
			return err
		}

		manifest, err := infraMedia.LoadManifest(
			mediaRoot,
		)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf(
			"Media: %s\n",
			mediaRoot,
		)
		fmt.Println()

		if len(manifest.Snapshots) == 0 {
			fmt.Println("No snapshots stored on this media.")
			return nil
		}

		fmt.Printf(
			"%-4s %-18s %-46s %10s\n",
			"#",
			"ID",
			"FILE",
			"SIZE",
		)

		fmt.Println(
			"--------------------------------------------------------------------------------",
		)

		for i, snapshot := range manifest.Snapshots {

			fmt.Printf(
				"%-4d %-18s %-46s %10s\n",
				i+1,
				snapshot.ID,
				snapshot.Filename,
				formatBytes(snapshot.Size),
			)
		}

		fmt.Println()
		fmt.Printf(
			"Total snapshots: %d\n",
			len(manifest.Snapshots),
		)

		return nil
	},
}

// ------------------------------------------------------------
// INSPECT
// ------------------------------------------------------------

var mediaInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a snapshot stored on external media",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		fmt.Println("InfraResc Media Inspect")
		fmt.Println()

		mediaRoot, err := chooseInfraRescMedia()
		if err != nil {
			return err
		}

		manifest, err := infraMedia.LoadManifest(
			mediaRoot,
		)
		if err != nil {
			return err
		}

		if len(manifest.Snapshots) == 0 {
			return fmt.Errorf(
				"no snapshots stored on this media",
			)
		}

		fmt.Println()
		index, err := chooseManifestSnapshot(
			manifest,
		)
		if err != nil {
			return err
		}

		snapshot := manifest.Snapshots[index]

		fmt.Println()
		fmt.Println("Snapshot")
		fmt.Println("----------------------------------------")
		fmt.Printf(
			"ID:          %s\n",
			snapshot.ID,
		)
		fmt.Printf(
			"Filename:    %s\n",
			snapshot.Filename,
		)
		fmt.Printf(
			"Size:        %s\n",
			formatBytes(snapshot.Size),
		)
		fmt.Printf(
			"SHA-256:     %s\n",
			snapshot.SHA256,
		)
		fmt.Printf(
			"Stored at:   %s\n",
			snapshot.StoredAt.Local().Format(
				"2006-01-02 15:04:05",
			),
		)

		fmt.Println()
		fmt.Printf(
			"Media:       %s\n",
			mediaRoot,
		)

		return nil
	},
}

// ------------------------------------------------------------
// VERIFY
// ------------------------------------------------------------

var mediaVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify snapshots stored on external media",

	RunE: func(
		cmd *cobra.Command,
		args []string,
	) error {

		fmt.Println("InfraResc Media Verify")
		fmt.Println()

		mediaRoot, err := chooseInfraRescMedia()
		if err != nil {
			return err
		}

		manifest, err := infraMedia.LoadManifest(
			mediaRoot,
		)
		if err != nil {
			return err
		}

		if len(manifest.Snapshots) == 0 {
			return fmt.Errorf(
				"no snapshots stored on this media",
			)
		}

		fmt.Println()

		index, err := chooseManifestSnapshot(
			manifest,
		)
		if err != nil {
			return err
		}

		snapshot := manifest.Snapshots[index]

		path := filepath.Join(
			infraMedia.SnapshotPath(mediaRoot),
			snapshot.Filename,
		)

		fmt.Println()
		fmt.Println("Verifying snapshot...")
		fmt.Println()

		actualHash, err := infraMedia.SHA256File(
			path,
		)
		if err != nil {
			return err
		}

		fmt.Printf(
			"Manifest SHA-256:\n%s\n\n",
			snapshot.SHA256,
		)

		fmt.Printf(
			"Actual SHA-256:\n%s\n\n",
			actualHash,
		)

		if !strings.EqualFold(
			snapshot.SHA256,
			actualHash,
		) {
			fmt.Println("Integrity: FAILED")

			return fmt.Errorf(
				"snapshot integrity verification failed",
			)
		}

		fmt.Println("Integrity: OK")

		return nil
	},
}

// ------------------------------------------------------------
// SNAPSHOT SELECTION
// ------------------------------------------------------------

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
			"%d. %s (%s)\n",
			i+1,
			file,
			formatBytes(info.Size()),
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

// ------------------------------------------------------------
// MEDIA SELECTION
// ------------------------------------------------------------

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

func chooseInfraRescMedia() (string, error) {

	media, err := infraMedia.DiscoverExternalMedia()
	if err != nil {
		return "", err
	}

	var repositories []string

	for _, root := range media {

		if infraMedia.IsInfraRescMedia(root) {
			repositories = append(
				repositories,
				root,
			)
		}
	}

	if len(repositories) == 0 {
		return "", fmt.Errorf(
			"no InfraResc media detected",
		)
	}

	fmt.Println("InfraResc media:")

	for i, root := range repositories {

		manifest, err := infraMedia.LoadManifest(root)
		if err != nil {
			fmt.Printf(
				"%d. %s (manifest error)\n",
				i+1,
				root,
			)
			continue
		}

		fmt.Printf(
			"%d. %s (%d snapshots)\n",
			i+1,
			root,
			len(manifest.Snapshots),
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

	if index < 1 || index > len(repositories) {
		return "", fmt.Errorf(
			"media selection out of range",
		)
	}

	return repositories[index-1], nil
}

func chooseManifestSnapshot(
	manifest *infraMedia.Manifest,
) (int, error) {

	fmt.Println("Snapshots:")
	fmt.Println()

	for i, snapshot := range manifest.Snapshots {

		fmt.Printf(
			"%d. %s (%s)\n",
			i+1,
			snapshot.Filename,
			formatBytes(snapshot.Size),
		)
	}

	fmt.Println()
	fmt.Print("Select snapshot: ")

	choice := readLine()

	index, err := strconv.Atoi(choice)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid snapshot selection",
		)
	}

	if index < 1 ||
		index > len(manifest.Snapshots) {
		return 0, fmt.Errorf(
			"snapshot selection out of range",
		)
	}

	return index - 1, nil
}

// ------------------------------------------------------------
// HELPERS
// ------------------------------------------------------------

func formatBytes(
	size int64,
) string {

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf(
			"%.2f GB",
			float64(size)/float64(GB),
		)

	case size >= MB:
		return fmt.Sprintf(
			"%.2f MB",
			float64(size)/float64(MB),
		)

	case size >= KB:
		return fmt.Sprintf(
			"%.2f KB",
			float64(size)/float64(KB),
		)

	default:
		return fmt.Sprintf(
			"%d B",
			size,
		)
	}
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
		mediaListCmd,
		mediaInspectCmd,
		mediaVerifyCmd,
	)

	rootCmd.AddCommand(
		mediaCmd,
	)
}
