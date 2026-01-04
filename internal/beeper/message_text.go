package beeper

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResolveMessageText produces a display string based on the chosen format.
func ResolveMessageText(rawMessage string, msgType string, textContent string, format MessageFormat) string {
	if format == FormatPlain {
		if strings.TrimSpace(textContent) != "" {
			return textContent
		}
		return extractMessageText(rawMessage, msgType, false)
	}

	rich := extractMessageText(rawMessage, msgType, true)
	if strings.TrimSpace(rich) != "" {
		return rich
	}
	return textContent
}

func extractMessageText(rawMessage string, msgType string, rich bool) string {
	if strings.TrimSpace(rawMessage) == "" {
		return ""
	}

	upperType := strings.ToUpper(strings.TrimSpace(msgType))
	if upperType == "" {
		upperType = "TEXT"
	}

	var payload any
	if err := json.Unmarshal([]byte(rawMessage), &payload); err != nil {
		return fallbackMessageText(rawMessage, upperType, rich)
	}

	switch value := payload.(type) {
	case map[string]any:
		return renderPayload(value, upperType, rich)
	case string:
		if upperType == "TEXT" {
			return value
		}
		return fallbackMessageText(value, upperType, rich)
	default:
		return fallbackMessageText(rawMessage, upperType, rich)
	}
}

func renderPayload(payload map[string]any, msgType string, rich bool) string {
	text := firstString(payload, "body", "text")
	if !rich || msgType == "TEXT" {
		return text
	}

	switch msgType {
	case "IMAGE":
		return formatWithOptionalText("[Image]", text)
	case "VIDEO":
		return formatWithOptionalText("[Video]", text)
	case "AUDIO":
		if url := firstString(payload, "url"); url != "" {
			return fmt.Sprintf("[Audio: %s]", url)
		}
		return "[Audio message]"
	case "FILE":
		filename := firstString(payload, "filename", "name")
		url := firstString(payload, "url")
		if filename != "" && url != "" {
			return fmt.Sprintf("[File: %s - %s]", filename, url)
		}
		if filename != "" {
			return fmt.Sprintf("[File: %s]", filename)
		}
		if url != "" {
			return fmt.Sprintf("[File: %s]", url)
		}
		return "[File]"
	case "LOCATION":
		geo := firstString(payload, "geo_uri", "geoUri")
		if geo != "" {
			return fmt.Sprintf("[Location: %s]", geo)
		}
		return "[Location]"
	case "CONTACT":
		name := firstString(payload, "display_name", "displayName", "name")
		if name != "" {
			return fmt.Sprintf("[Contact: %s]", name)
		}
		return "[Contact]"
	case "STICKER":
		if url := firstString(payload, "url"); url != "" {
			return fmt.Sprintf("[Sticker: %s]", url)
		}
		return "[Sticker]"
	default:
		return fallbackMessageText(text, msgType, rich)
	}
}

func fallbackMessageText(value string, msgType string, rich bool) string {
	if msgType == "TEXT" {
		return value
	}
	if !rich {
		return value
	}
	if strings.TrimSpace(value) != "" && msgType != "" {
		return formatWithOptionalText(fmt.Sprintf("[%s]", msgType), value)
	}
	if msgType == "" {
		msgType = "MESSAGE"
	}
	return fmt.Sprintf("[%s]", msgType)
}

func formatWithOptionalText(prefix string, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return prefix
	}
	return fmt.Sprintf("%s %s", prefix, text)
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			s, ok := value.(string)
			if ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
