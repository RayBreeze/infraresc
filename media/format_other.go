//go:build !windows

package media

import "fmt"

func FormatExternalMedia(mediaRoot string) error {
	return fmt.Errorf("external media formatting is only supported on Windows")
}
