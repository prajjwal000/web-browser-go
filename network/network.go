package network

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type Header_map map[string]string

type request struct {
	Host    string
	Scheme  string
	Headers Header_map
	Port    int
	Path    string
}

type response struct {
	Status  string
	Headers Header_map
	Body    string
}

func Parse(url string) (request, error) {
	req := request{}
	req.Headers = make(map[string]string)
	var ok bool
	var temp string
	var err error

	req.Scheme, req.Host, ok = strings.Cut(url, "://")
	if !ok {
		return req, errors.New("Error: Invalid Url")
	}
	if req.Scheme != "http" && req.Scheme != "https" && req.Scheme != "file"{
		return req, errors.New("Error: Unknown Scheme")
	}

	if req.Scheme == "file" {
		return req, nil
	}

	req.Host, temp, ok = strings.Cut(req.Host, ":")
	if !ok {
		if req.Scheme == "http" {
			req.Port = 80
		} else {
			req.Port = 443
		}
		req.Host, req.Path, _ = strings.Cut(req.Host, "/")
		_, req.Host, _ = strings.Cut(req.Host, "www.")
		req.Path = "/" + req.Path
		req.Add_header("Host", req.Host)
		req.Add_header("Connection", "close")
		req.Add_header("User-Agent", "botted")
		return req, nil
	}

	temp, req.Path, _ = strings.Cut(temp, "/")
	req.Port, err = strconv.Atoi(temp)
	req.Path = "/" + req.Path
	_, req.Host, _ = strings.Cut(req.Host, "www.")
	req.Add_header("Host", req.Host)
	req.Add_header("Connection", "close")
	req.Add_header("User-Agent", "botted")

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

	if req.Scheme == "file" {
		file, err := os.Open(req.Host)
		if err != nil {
			log.Println(err)
			return resp
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			log.Println(err)
			return resp
		}
		resp.Status = "HTTP/1.1 200 OK"
		resp.Body = string(content)
		return resp
	}

	buf := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	buf.WriteString(req.http_raw())
	buf.Flush()

	resp.read(buf)

	return resp
}

func (req *request) Add_header(field, value string) {
	req.Headers[field] = value
}

func (req request) http_raw() string {
	var ret strings.Builder
	ret.WriteString("GET " + req.Path + " HTTP/1.1\r\n")
	for k, v := range req.Headers {
		ret.WriteString(k + ": " + v + "\r\n")
	}
	ret.WriteString("\r\n")
	return ret.String()
}

func (resp *response) read(buf *bufio.ReadWriter) {

	statusLine, _, err := buf.ReadLine()
	if err != nil {
		log.Println(err)
	}
	resp.Status = string(statusLine)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			log.Println(err)
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
		}
		body.Write(line)
		body.WriteString("\n")
	}

	resp.Body = body.String()

}
