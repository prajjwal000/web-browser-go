package network

import (
	"errors"
	"strconv"
	"strings"
)

type request struct {
	host   string
	scheme string
	port   int
	path   string
}

func Parse(url string) (request, error) {
	req := request{}
	var ok bool
	var temp string
	var err error

	req.scheme, req.host, ok = strings.Cut(url, "://")
	if !ok {
		return req, errors.New("Error: Invalid Url")
	}
	if req.scheme != "http" && req.scheme != "https" {
		return req, errors.New("Error: Unknown scheme")
	}

	req.host, temp, ok = strings.Cut(req.host, ":")
	if !ok {
		if req.scheme == "http" {
			req.port = 80
		} else {
			req.port = 443
		}
		req.host, req.path, _ = strings.Cut(req.host, "/")
		req.path = "/" + req.path
		return req, nil
	}

	temp, req.path, _ = strings.Cut(temp, "/")
	req.port, err = strconv.Atoi(temp)
	req.path = "/" + req.path

	return req, err
}
