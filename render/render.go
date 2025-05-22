package render

import (
	"fmt"
	"strings"

	"github.com/prajjwal000/web-browser-go/network"
)

func Render(resp network.Response) {
	if resp.Scheme == "view-source" {
		fmt.Print(resp.Body)
		return
	}

	content := stripHTML(resp.Body)
	content = decodeHTMLEntities(content)
	fmt.Print(content)
}

func stripHTML(content string) string {
	var builder strings.Builder
	inTag := false

	for _, char := range content {
		switch char {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				builder.WriteRune(char)
			}
		}
	}

	return builder.String()
}

func decodeHTMLEntities(content string) string {
	replacements := map[string]string{
		"&lt;":   "<",
		"&gt;":   ">",
		"&amp;":  "&",
		"&quot;": "\"",
		"&apos;": "'",
	}

	for entity, char := range replacements {
		content = strings.ReplaceAll(content, entity, char)
	}

	return content
}
