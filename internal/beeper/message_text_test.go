package beeper

import "testing"

func TestResolveMessageTextRich(t *testing.T) {
	raw := `{"url":"https://example.com/file.pdf","filename":"report.pdf"}`
	text := ResolveMessageText(raw, "FILE", "", FormatRich)
	if text != "[File: report.pdf - https://example.com/file.pdf]" {
		t.Fatalf("unexpected rich text: %s", text)
	}

	plain := ResolveMessageText(raw, "FILE", "", FormatPlain)
	if plain != "" {
		t.Fatalf("unexpected plain text: %s", plain)
	}
}
