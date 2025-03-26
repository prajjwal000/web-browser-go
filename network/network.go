package network

import (
	"bufio"
	"compress/gzip"
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
	"time"
)

type HeaderMap map[string]string

type Request struct {
	Host          string
	Scheme        string
	Headers       HeaderMap
	Port          int
	Method        string
	Path          string
	Conn          *net.Conn
	ResponseCache ResponseCache
	Redirect      int
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
	req.Headers["Accept-Encoding"] = "gzip"

	return req, nil
}

func (req *Request) Send() (Response, error) {
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

func (req *Request) sendViewSource() (Response, error) {
	tempReq := req
	tempReq.Scheme = "https"
	resp, err := tempReq.Send()
	if err != nil {
		return Response{}, err
	}
	resp.Scheme = "view-source"
	return resp, nil
}

func (req *Request) sendNet() (Response, error) {

	if req.ResponseCache.TimeToLive[req.Path] > time.Now().Unix() {
		log.Println("Cache hit for:", req.Path)
		return req.ResponseCache.Cache[req.Path], nil
	}

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

	if resp.Headers["Cache-Control"] != "" {
		cacheControl := resp.Headers["Cache-Control"]
		if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") {
			log.Println("Response is not cacheable due to Cache-Control header")
			return resp, nil
		}
		var maxAge int64 = 0
		if strings.Contains(cacheControl, "max-age") {
			maxAgeStr := strings.Split(cacheControl, "max-age=")[1]
			maxAgeStr = strings.Split(maxAgeStr, ",")[0]
			maxAge, err = strconv.ParseInt(maxAgeStr, 10, 64)
			if err != nil {
				return resp, fmt.Errorf("invalid max-age in Cache-Control header: %w", err)
			}
			req.CacheResponse(resp, maxAge)
			log.Println("Response cached for", maxAge, "seconds")
		}
	}

	return resp, nil
}

func (req *Request) handleRedirect(resp Response, location string) (Response, error) {
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
		req.Host = newReq.Host
		req.Port = newReq.Port
		req.Scheme = newReq.Scheme
		req.Headers = newReq.Headers
		req.Method = newReq.Method
		req.Conn = newReq.Conn
		req.Path = newReq.Path
		req.ResponseCache = newReq.ResponseCache
		req.Headers["Host"] = req.Host
		req.Headers["Connection"] = "keep-alive"
		req.Headers["User-Agent"] = "web-browser-go"
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

func (req *Request) sendData() (Response, error) {
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

	// Read headers
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			return fmt.Errorf("failed to read header: %w", err)
		}
		if len(line) == 0 {
			break
		}
		key, value, _ := strings.Cut(string(line), ": ")
		resp.Headers[key] = strings.TrimSpace(value)
	}

	transferEncoding := strings.ToLower(resp.Headers["Transfer-Encoding"])
	isChunked := transferEncoding == "chunked"

	// Handle body
	if isChunked {
		var body strings.Builder
		for {
			chunkSizeLine, _, err := buf.ReadLine()
			if err != nil {
				return fmt.Errorf("failed to read chunk size: %w", err)
			}

			chunkSizeStr := strings.TrimSpace(string(chunkSizeLine))
			chunkSize, err := strconv.ParseInt(chunkSizeStr, 16, 64)
			if err != nil {
				return fmt.Errorf("invalid chunk size: %w", err)
			}

			if chunkSize == 0 {
				_, _, err = buf.ReadLine()
				if err != nil {
					return fmt.Errorf("failed to read final CRLF: %w", err)
				}
				break
			}

			chunkData := make([]byte, chunkSize)
			_, err = io.ReadFull(buf, chunkData)
			if err != nil {
				return fmt.Errorf("failed to read chunk data: %w", err)
			}
			body.Write(chunkData)

			_, _, err = buf.ReadLine()
			if err != nil {
				return fmt.Errorf("failed to read chunk CRLF: %w", err)
			}
		}
		resp.Body = body.String()
	} else {
		var body strings.Builder
		hasContentLength := resp.Headers["Content-Length"] != ""

		if hasContentLength {
			// Read exactly Content-Length bytes
			contentLength, err := strconv.Atoi(resp.Headers["Content-Length"])
			if err != nil {
				return fmt.Errorf("invalid Content-Length: %w", err)
			}

			bodyData := make([]byte, contentLength)
			_, err = io.ReadFull(buf, bodyData)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to read body: %w", err)
			}
			body.Write(bodyData)
		} else {
			bodyData, err := io.ReadAll(buf)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to read body: %w", err)
			}
			body.Write(bodyData)
		}
		resp.Body = body.String()
	}

	// Handle gzip encoding
	if resp.Headers["Content-Encoding"] == "gzip" || transferEncoding == "gzip" {
		gzipReader, err := gzip.NewReader(strings.NewReader(resp.Body))
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		uncompressedBody, err := io.ReadAll(gzipReader)
		if err != nil {
			return fmt.Errorf("failed to read gzip body: %w", err)
		}
		resp.Body = string(uncompressedBody)
	}

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
