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

	for range 10 {
		resp, err := req.Send()
		if err != nil {
			fmt.Printf("Error making request: %v\n", err)
			os.Exit(2)
		}

		render.Render(resp)
		time.Sleep(1 * time.Second)
	}

}
