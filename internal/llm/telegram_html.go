package llm

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var allowedTelegramTagsRegex *regexp.Regexp

func init() {
	// Telegram supported tags list
	allowedTags := []string{
		"b", "strong", "i", "em", "u", "ins", "s", "strike", "del", "code", "pre",
	}

	var allowRegexParts []string
	for _, tag := range allowedTags {
		allowRegexParts = append(allowRegexParts, fmt.Sprintf("</?%s>", tag))
	}
	// Special handling for a href - only allow http/https URLs to prevent XSS
	allowRegexParts = append(allowRegexParts, `<a href="(https?:\/\/[^"]*)">`)
	allowRegexParts = append(allowRegexParts, `<a href='(https?:\/\/[^']*)'>`)
	allowRegexParts = append(allowRegexParts, `</a>`)
	// Special handling for tg-spoiler
	allowRegexParts = append(allowRegexParts, `<tg-spoiler>`)
	allowRegexParts = append(allowRegexParts, `</tg-spoiler>`)

	allowPattern := strings.Join(allowRegexParts, "|")
	allowedTelegramTagsRegex = regexp.MustCompile("(?i)(" + allowPattern + ")")
}

// SanitizeTelegramHTML ensures that only permitted Telegram HTML tags remain
// and that any raw `<` or `&` inside text nodes are properly escaped.
func SanitizeTelegramHTML(input string) string {
	// Split input by allowed tags
	parts := allowedTelegramTagsRegex.Split(input, -1)
	tags := allowedTelegramTagsRegex.FindAllString(input, -1)

	var result strings.Builder
	for i, part := range parts {
		// Escape &, <, > in text parts
		escapedPart := strings.ReplaceAll(part, "&", "&amp;")
		escapedPart = strings.ReplaceAll(escapedPart, "<", "&lt;")
		escapedPart = strings.ReplaceAll(escapedPart, ">", "&gt;")
		result.WriteString(escapedPart)

		// Re-append the allowed tag
		if i < len(tags) {
			result.WriteString(tags[i])
		}
	}

	return result.String()
}

// ValidSanitizedTelegramHTML checks tag structure in the sanitizer's output
// and requires some message text. It uses Telegram's entity nesting rules.
// https://core.telegram.org/bots/api#formatting-options
func ValidSanitizedTelegramHTML(input string) bool {
	if strings.ContainsRune(input, '\x00') {
		return false
	}

	var openTags []string
	hasText := false
	textStart := 0
	for _, bounds := range allowedTelegramTagsRegex.FindAllStringIndex(input, -1) {
		start, end := bounds[0], bounds[1]
		between := input[textStart:start]
		if strings.ContainsAny(between, "<>") {
			return false
		}
		hasText = hasText || strings.TrimSpace(between) != ""
		textStart = end

		// The regexp consumes the whole tag, including quoted href values.
		tag := input[start+1 : end-1]
		name, closing := strings.CutPrefix(tag, "/")
		name, _, _ = strings.Cut(name, " ")
		name = strings.ToLower(name)

		parent := ""
		if len(openTags) > 0 {
			parent = openTags[len(openTags)-1]
		}
		if closing {
			if name != parent {
				return false
			}
			if name == "code" && len(openTags) > 1 {
				// A code wrapper inside pre must span the entire block.
				const endPre = "</pre>"
				if len(input)-end < len(endPre) || !strings.EqualFold(input[end:end+len(endPre)], endPre) {
					return false
				}
			}
			openTags = openTags[:len(openTags)-1]
			continue
		}

		switch name {
		case "code", "pre":
			if parent != "" && !(name == "code" && parent == "pre" && between == "") {
				return false
			}
		default:
			// Links can contain styles, but cannot nest even through a style.
			if parent == "code" || parent == "pre" || (name == "a" && slices.Contains(openTags, "a")) {
				return false
			}
		}
		openTags = append(openTags, name)
	}

	tail := input[textStart:]
	return len(openTags) == 0 && !strings.ContainsAny(tail, "<>") && (hasText || strings.TrimSpace(tail) != "")
}
