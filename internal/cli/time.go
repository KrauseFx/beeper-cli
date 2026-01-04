package cli

import (
	"fmt"
	"strings"
	"time"
)

func parseTimeFlag(value string, days int) (*time.Time, error) {
	if strings.TrimSpace(value) != "" {
		return parseTimePtr(value)
	}
	if days > 0 {
		t := time.Now().AddDate(0, 0, -days)
		return &t, nil
	}
	return nil, nil
}

func parseTimePtr(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("invalid time %q: use RFC3339", value)
	}
	return &parsed, nil
}

func parseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", value, err)
	}
	return d, nil
}
