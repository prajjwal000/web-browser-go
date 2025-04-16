package render

import (
	"fmt"
	"strings"

	neti "github.com/prajjwal000/web-browser-go/network"
)

func Render(resp neti.Response) {
	in_tag := false
	var builder strings.Builder
	for _, char := range resp.Body {
		if char == '<' {
			in_tag = true
		} else if char == '>' {
			in_tag = false
		} else if in_tag == false {
			builder.WriteRune(char)
		}
	}
	data := builder.String()
	data = strings.ReplaceAll(data, "&lt", "<")
	data = strings.ReplaceAll(data, "&gt", ">")
	fmt.Print(data)
}
