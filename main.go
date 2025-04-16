package main

import (
	"fmt"
	"os"

	neti "github.com/prajjwal000/web-browser-go/network"
	render "github.com/prajjwal000/web-browser-go/render"
)

func main() {
	url := "file://test.html" 
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	req, err := neti.Parse(url)
	if err != nil {
		fmt.Printf("Error parsing URL: %v\n", err)
		return
	}

	resp, err := req.Get()
	if err != nil {
		fmt.Print(err)
		return
	}

	render.Render(resp)
}
