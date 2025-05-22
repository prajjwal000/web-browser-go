package main

import (
	"fmt"
	"os"
	"time"

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
		os.Exit(1)
	}

	render.Render(resp)

	resp2, err := req.Get()
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(10)
	}

	render.Render(resp2)

}
