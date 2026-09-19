//go:build windows

package media

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func FormatExternalMedia(mediaRoot string) error {
	root := filepath.Clean(mediaRoot)

	if len(root) < 2 || root[1] != ':' {
		return fmt.Errorf(
			"invalid Windows drive path: %s",
			mediaRoot,
		)
	}

	driveLetter := strings.ToUpper(
		string(root[0]),
	)

	if driveLetter[0] < 'A' ||
		driveLetter[0] > 'Z' {
		return fmt.Errorf(
			"invalid drive letter: %s",
			driveLetter,
		)
	}

	fmt.Printf(
		"Formatting %s: as exFAT...\n",
		driveLetter,
	)

	command := fmt.Sprintf(
		"Format-Volume -DriveLetter '%s' -FileSystem exFAT -NewFileSystemLabel 'INFRARESC' -Confirm:$false",
		driveLetter,
	)

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		command,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf(
			"formatting drive %s: %w\n%s",
			driveLetter,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}
