package main

import (
	"fmt"

	neti "github.com/prajjwal000/web-browser-go/network"
) 

func main() {
	req, _ := neti.Parse("https://www.example.com/")
	fmt.Print(req.Get())
}
