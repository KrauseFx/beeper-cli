package cli

import (
	"fmt"
	"strings"

	"github.com/KrauseFx/beeper-cli/internal/beeper"
)

func parseMessageFormat(value string) (beeper.MessageFormat, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", string(beeper.FormatRich):
		return beeper.FormatRich, nil
	case string(beeper.FormatPlain):
		return beeper.FormatPlain, nil
	default:
		return "", fmt.Errorf("invalid format %q: use plain or rich", value)
	}
}
