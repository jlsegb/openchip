package validate

import (
	"fmt"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$`)

func ChipID(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("chip id is required")
	}
	if len(value) > 50 {
		return fmt.Errorf("chip id must be 50 characters or fewer")
	}
	return nil
}

func Email(value string) error {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > 320 || !emailRegex.MatchString(value) {
		return fmt.Errorf("a valid email is required")
	}
	return nil
}

func RequiredText(field, value string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(value) > max {
		return fmt.Errorf("%s must be %d characters or fewer", field, max)
	}
	return nil
}

func OptionalText(field string, value *string, max int) error {
	if value == nil {
		return nil
	}
	if len(strings.TrimSpace(*value)) > max {
		return fmt.Errorf("%s must be %d characters or fewer", field, max)
	}
	return nil
}

func Species(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dog", "cat", "other":
		return nil
	default:
		return fmt.Errorf("species must be one of dog, cat, or other")
	}
}
