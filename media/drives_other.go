//go:build !windows

package media

import "fmt"

func DiscoverExternalMedia() ([]string, error) {
	return nil, fmt.Errorf("external media discovery is only supported on Windows")
}
