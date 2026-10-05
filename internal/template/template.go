// Package template renders poll text templates.
package template

import (
	"fmt"
	"strings"
)

// Render substitutes the supported placeholders in text.
// A doubled brace escapes a brace, and the only supported placeholder is {end_date}.
func Render(text, endDate string) (string, error) {
	var rendered strings.Builder
	rendered.Grow(len(text))

	for index := 0; index < len(text); {
		switch text[index] {
		case '{':
			if index+1 < len(text) && text[index+1] == '{' {
				rendered.WriteByte('{')
				index += 2
				continue
			}

			closingBrace := strings.IndexByte(text[index+1:], '}')
			if closingBrace < 0 {
				return "", fmt.Errorf("template contains an unmatched opening brace")
			}
			name := text[index+1 : index+1+closingBrace]
			if name != "end_date" {
				return "", fmt.Errorf("template contains unsupported placeholder %q", name)
			}
			rendered.WriteString(endDate)
			index += closingBrace + 2
		case '}':
			if index+1 < len(text) && text[index+1] == '}' {
				rendered.WriteByte('}')
				index += 2
				continue
			}
			return "", fmt.Errorf("template contains an unmatched closing brace")
		default:
			rendered.WriteByte(text[index])
			index++
		}
	}

	result := rendered.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("rendered text must not be empty")
	}
	return result, nil
}
