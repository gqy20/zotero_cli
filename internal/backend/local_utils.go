package backend

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func formatAttachmentLinkMode(mode int) string {
	switch mode {
	case 0:
		return "imported_file"
	case 1:
		return "imported_url"
	case 2:
		return "linked_file"
	case 3:
		return "linked_url"
	default:
		return fmt.Sprintf("mode_%d", mode)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return stringsJoinFields(value)
}

func notePreview(value string) string {
	text := stripHTMLTags(value)
	text = normalizeWhitespace(text)
	if len(text) <= 80 {
		return text
	}
	return text[:77] + "..."
}

func stripHTMLTags(value string) string {
	var b strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		case '\n', '\r', '\t':
			if !inTag {
				b.WriteRune(' ')
			}
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

var (
	localDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	localTimePattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
)

func normalizeLocalDate(value string) string {
	value = normalizeWhitespace(value)
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	if len(parts) >= 3 && localDatePattern.MatchString(parts[0]) && parts[1] == parts[0] && localTimePattern.MatchString(parts[2]) {
		return parts[0]
	}
	if len(parts) >= 2 && localDatePattern.MatchString(parts[0]) && parts[1] == parts[0] {
		return parts[0]
	}
	return value
}

func stringsJoinFields(value string) string {
	parts := stringsFields(value)
	if len(parts) == 0 {
		return ""
	}
	return joinWithSpace(parts)
}

func stringsFields(value string) []string {
	return strings.Fields(value)
}

func joinWithSpace(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result += " " + part
	}
	return result
}

func stringsCutPrefix(value string, prefix string) (string, bool) {
	if len(value) < len(prefix) || value[:len(prefix)] != prefix {
		return "", false
	}
	return value[len(prefix):], true
}

func requireDir(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %s", label, path)
	}
	return nil
}

func requireFile(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is not a file: %s", label, path)
	}
	return nil
}
