//go:build windows

package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func DiscoverExternalMedia() ([]string, error) {

	var result []string

	for letter := 'A'; letter <= 'Z'; letter++ {

		root := fmt.Sprintf(
			"%c:\\",
			letter,
		)

		if _, err := os.Stat(root); err != nil {
			continue
		}

		pathPtr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}

		driveType := windows.GetDriveType(
			pathPtr,
		)

		if driveType != windows.DRIVE_REMOVABLE {
			continue
		}

		result = append(
			result,
			filepath.Clean(
				strings.TrimSpace(root),
			),
		)
	}

	return result, nil
}
