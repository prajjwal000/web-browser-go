package main

import (
	"fmt"

	net "github.com/prajjwal000/web-browser-go/network"
) 

func main() {
	req, _ := net.Parse("http://www.example.com/lovely")
	fmt.Print(req)
}
