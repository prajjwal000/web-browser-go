package network

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
)

type HeaderMap map[string]string

type Request struct {
	Host     string
	Scheme   string
	Headers  HeaderMap
	Port     int
	Method   string
	Path     string
	Conn     *net.Conn
	Redirect int
}

type Response struct {
	Scheme  string
	Status  string
	Headers HeaderMap
	Body    string
}

func Parse(url string) (Request, error) {
	req := Request{
		Headers: make(HeaderMap),
	}

	scheme, rest, ok := strings.Cut(url, "://")
	if !ok {
		if scheme, rest, ok = strings.Cut(url, ":"); ok && scheme == "data" {
			if host, path, ok := strings.Cut(rest, ","); ok {
				req.Scheme = scheme
				req.Host = host
				req.Path = path
				return req, nil
			}
			return req, errors.New("invalid data URL format")
		}
		return req, errors.New("invalid URL format")
	}

	if !isValidScheme(scheme) {
		return req, fmt.Errorf("unsupported scheme: %s", scheme)
	}
	req.Scheme = scheme

	if req.Scheme == "file" {
		req.Host = rest
		return req, nil
	}

	host := strings.TrimPrefix(rest, "www.")
	host, portStr, ok := strings.Cut(host, ":")
	if !ok {
		req.Port = defaultPort(scheme)
		host, req.Path, _ = strings.Cut(host, "/")
		req.Host = host
		req.Path = "/" + req.Path
	} else {
		portStr, req.Path, _ = strings.Cut(portStr, "/")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return req, fmt.Errorf("invalid port: %w", err)
		}
		req.Port = port
		req.Host = host
		req.Path = "/" + req.Path
	}

	req.Method = "GET"
	req.Headers["Host"] = req.Host
	req.Headers["Connection"] = "keep-alive"
	req.Headers["User-Agent"] = "web-browser-go"

	return req, nil
}

func (req Request) Send() (Response, error) {
	switch req.Scheme {
	case "view-source":
		return req.sendViewSource()
	case "http", "https":
		return req.sendNet()
	case "file":
		return req.sendFile()
	case "data":
		return req.sendData()
	default:
		return Response{}, fmt.Errorf("unsupported scheme: %s", req.Scheme)
	}
}

func (req Request) sendViewSource() (Response, error) {
	tempReq := req
	tempReq.Scheme = "https"
	resp, err := tempReq.Send()
	if err != nil {
		return Response{}, err
	}
	resp.Scheme = "view-source"
	return resp, nil
}

func (req Request) sendNet() (Response, error) {
	resp := Response{
		Headers: make(HeaderMap),
		Scheme:  req.Scheme,
	}

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)

	var conn *net.Conn
	var err error

	if req.Conn != nil {
		conn = req.Conn
	} else {
		conn, err = dial(req.Scheme, addr)
		if err != nil {
			return resp, fmt.Errorf("failed to connect: %w", err)
		}
	}
	req.Conn = conn

	if err := req.write(conn); err != nil {
		return resp, fmt.Errorf("failed to write request: %w", err)
	}

	if err := resp.read(conn); err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}

	location := resp.Headers["Location"]
	if location != "" {
		return req.handleRedirect(resp, location)
	}

	req.Redirect = 0

	return resp, nil
}

func (req Request) handleRedirect(resp Response, location string) (Response, error) {
	if req.Redirect >= 5 {
		return resp, fmt.Errorf("too many redirects")
	}
	req.Redirect++
	log.Println("Redirecting to:", location)

	if location[0] == '/' {
		req.Path = location
		return req.sendNet()
	}

	newReq, err := Parse(location)
	if err != nil {
		return resp, fmt.Errorf("failed to parse redirect URL: %w", err)
	}
	if newReq.Host != "" && newReq.Host != req.Host && newReq.Port != req.Port {
		newReq.Redirect = req.Redirect
		req = newReq
		return req.Send()
	}

	req.Path = newReq.Path

	return req.sendNet()
}

func dial(scheme, addr string) (*net.Conn, error) {
	if scheme == "https" {
		conn, err := (&tls.Dialer{
			Config: &tls.Config{
				InsecureSkipVerify: true,
			},
		}).Dial("tcp", addr)
		return &conn, err
	}
	conn, err := (&net.Dialer{}).Dial("tcp", addr)
	return &conn, err
}

func (req Request) sendFile() (Response, error) {
	resp := Response{
		Headers: make(HeaderMap),
		Scheme:  req.Scheme,
	}

	file, err := os.Open(req.Host)
	if err != nil {
		return resp, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return resp, fmt.Errorf("failed to read file: %w", err)
	}

	resp.Status = "HTTP/1.1 200 OK"
	resp.Body = string(content)
	return resp, nil
}

func (req Request) sendData() (Response, error) {
	resp := Response{
		Headers: make(HeaderMap),
		Scheme:  req.Scheme,
		Status:  "HTTP/1.1 200 OK",
	}
	resp.Headers["Content-Type"] = req.Host
	resp.Body = req.Path
	return resp, nil
}

func (req Request) httpRaw() string {
	var ret strings.Builder
	ret.WriteString(req.Method + " " + req.Path + " HTTP/1.1\r\n")

	headers := []string{"Host", "User-Agent", "Connection"}
	for _, header := range headers {
		if value, exists := req.Headers[header]; exists {
			ret.WriteString(header + ": " + value + "\r\n")
		}
	}

	for k, v := range req.Headers {
		if !slices.Contains(headers, k) {
			ret.WriteString(k + ": " + v + "\r\n")
		}
	}

	ret.WriteString("\r\n")
	return ret.String()
}

func (req Request) write(conn *net.Conn) error {
	buf := bufio.NewWriter(*conn)
	if _, err := buf.WriteString(req.httpRaw()); err != nil {
		return err
	}
	return buf.Flush()
}

func (resp *Response) read(conn *net.Conn) error {
	buf := bufio.NewReader(*conn)
	statusLine, _, err := buf.ReadLine()
	if err != nil {
		return fmt.Errorf("failed to read status line: %w", err)
	}
	resp.Status = string(statusLine)

	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			return fmt.Errorf("failed to read header: %w", err)
		}
		if len(line) == 0 {
			break
		}
		key, value, _ := strings.Cut(string(line), ": ")
		resp.Headers[key] = strings.Trim(value, " ")
	}

	content_length := 1000000
	if resp.Headers["Content-Length"] != "" {
		content_length, err = strconv.Atoi(resp.Headers["Content-Length"])
	}
	var body strings.Builder
	for content_length > body.Len() {
		byte, err := buf.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}
		body.WriteByte(byte)
	}
	resp.Body = body.String()
	return nil
}

func isValidScheme(scheme string) bool {
	validSchemes := []string{"http", "https", "file", "view-source", "data"}
	return slices.Contains(validSchemes, scheme)
}

func defaultPort(scheme string) int {
	if scheme == "http" {
		return 80
	}
	return 443
}
