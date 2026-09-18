package auth

import (
	"bufio"
	"fmt"
	"strings"
)

func InteractiveConfigure(
	reader *bufio.Reader,
) (AWSProfile, error) {

	fmt.Println()
	fmt.Println("InfraResc AWS Authentication Configuration")
	fmt.Println()

	profileName := prompt(
		reader,
		"Profile name",
		"infraresc",
	)

	region := promptRequired(
		reader,
		"AWS Region",
	)

	profile := AWSProfile{
		Name:   profileName,
		Region: region,
	}

	if err := WriteAWSProfile(profile); err != nil {
		return AWSProfile{}, err
	}

	return profile, nil
}

func prompt(
	reader *bufio.Reader,
	label string,
	defaultValue string,
) string {

	fmt.Printf(
		"%s [%s]: ",
		label,
		defaultValue,
	)

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	return input
}

func promptRequired(
	reader *bufio.Reader,
	label string,
) string {

	for {

		fmt.Printf("%s: ", label)

		input, err := reader.ReadString('\n')
		if err != nil {
			return ""
		}

		input = strings.TrimSpace(input)

		if input != "" {
			return input
		}

		fmt.Println("This value is required.")
	}
}
