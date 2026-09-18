package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AWSProfile struct {
	Name   string
	Region string
}

func awsConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	return filepath.Join(home, ".aws", "config"), nil
}

func WriteAWSProfile(profile AWSProfile) error {
	if strings.TrimSpace(profile.Name) == "" {
		return errors.New("profile name cannot be empty")
	}

	if strings.TrimSpace(profile.Region) == "" {
		return errors.New("AWS region cannot be empty")
	}

	path, err := awsConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read AWS config: %w", err)
	}

	content := removeProfileBlock(
		string(existing),
		profile.Name,
	)

	content = strings.TrimSpace(content)

	if content != "" {
		content += "\n\n"
	}

	content += fmt.Sprintf(
		"[profile %s]\nregion = %s\n",
		profile.Name,
		profile.Region,
	)

	if err := os.WriteFile(
		path,
		[]byte(content),
		0600,
	); err != nil {
		return fmt.Errorf("failed to write AWS config: %w", err)
	}

	return nil
}

func removeProfileBlock(
	content string,
	profileName string,
) string {

	lines := strings.Split(content, "\n")

	target := "[profile " + profileName + "]"

	var result []string
	skip := false

	for _, line := range lines {

		trimmed := strings.TrimSpace(line)

		if strings.EqualFold(trimmed, target) {
			skip = true
			continue
		}

		if skip && strings.HasPrefix(trimmed, "[") {
			skip = false
		}

		if !skip {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func ProfileExists(profileName string) bool {

	path, err := awsConfigPath()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	target := "[profile " + profileName + "]"

	return strings.Contains(
		string(data),
		target,
	)
}
