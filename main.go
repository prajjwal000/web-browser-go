package main

import (
	"fmt"
	"os"

	"github.com/prajjwal000/web-browser-go/network"
	"github.com/prajjwal000/web-browser-go/render"
)

func main() {
	url := "file://test.html"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	req, err := network.Parse(url)
	if err != nil {
		fmt.Printf("Error parsing URL: %v\n", err)
		os.Exit(1)
	}

	resp, err := req.Get()
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(2)
	}

	render.Render(resp)
}
