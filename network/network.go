package network

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

type request struct {
	Host   string
	Scheme string
	Port   int
	Path   string
}

type response struct {
	Status  string
	Headers map[string]string
	Body    string
}

func Parse(url string) (request, error) {
	req := request{}
	var ok bool
	var temp string
	var err error

	req.Scheme, req.Host, ok = strings.Cut(url, "://")
	if !ok {
		return req, errors.New("Error: Invalid Url")
	}
	if req.Scheme != "http" && req.Scheme != "https" {
		return req, errors.New("Error: Unknown Scheme")
	}

	req.Host, temp, ok = strings.Cut(req.Host, ":")
	if !ok {
		if req.Scheme == "http" {
			req.Port = 80
		} else {
			req.Port = 443
		}
		req.Host, req.Path, _ = strings.Cut(req.Host, "/")
		req.Path = "/" + req.Path
		return req, nil
	}

	temp, req.Path, _ = strings.Cut(temp, "/")
	_, req.Host, _ = strings.Cut(req.Host, "www.")
	req.Port, err = strconv.Atoi(temp)
	req.Path = "/" + req.Path

	return req, err
}

func (req request) Get() response {
	resp := response{}
	resp.Headers = make(map[string]string)
	var conn net.Conn
	var err error
	if req.Scheme == "https" {
		conf := &tls.Config{}
		conn, err = tls.Dial("tcp", req.Host+":"+strconv.Itoa(req.Port), conf)
		if err != nil {
			log.Println(err)
			return resp
		}
		defer conn.Close()
	}

	if req.Scheme == "http" {
		conn, err = net.Dial("tcp", req.Host+":"+strconv.Itoa(req.Port))
		if err != nil {
			log.Println(err)
			return resp
		}
		defer conn.Close()
	}

	buf := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	buf.WriteString("GET " + req.Path + " HTTP/1.1\r\n" +
		"Host: " + req.Host + "\r\n" +
		"Connection: close\r\n\r\n")
	buf.Flush()

	statusLine, _, err := buf.ReadLine()
	if err != nil {
		log.Println(err)
		return resp
	}
	resp.Status = string(statusLine)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			log.Println(err)
			return resp
		}
		if len(line) == 0 {
			break
		}
		key, value, _ := strings.Cut(string(line), ": ")
		resp.Headers[key] = value
	}

	var body strings.Builder
	for {
		line, _, err := buf.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			return resp
		}
		body.Write(line)
		body.WriteString("\n")
	}

	resp.Body = body.String()

	return resp
}
