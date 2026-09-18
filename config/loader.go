package config

import (
	"os"
	"strings"
)

func Load() Config {

	cfg := Default()

	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		cfg.Profile = profile
	}

	if region := os.Getenv("AWS_REGION"); region != "" {
		cfg.Region = region
	}

	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		cfg.Region = region
	}

	return cfg
}

func ResolveProfile(profile string) string {

	if strings.TrimSpace(profile) != "" {
		return profile
	}

	return Load().Profile
}
