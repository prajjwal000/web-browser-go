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

type Request struct {
	Host    string
	Scheme  string
	Headers Header_map
	Port    int
	Path    string
}

type Response struct {
	Scheme  string
	Status  string
	Headers Header_map
	Body    string
}

func Parse(url string) (Request, error) {
	req := Request{}
	req.Headers = make(map[string]string)
	var ok bool
	var temp string
	var err error

	req.Scheme, req.Host, ok = strings.Cut(url, "://")
	if !ok {
		if req.Scheme, req.Host, ok = strings.Cut(url, ":"); ok && req.Scheme == "data" {
			if req.Host, req.Path, ok = strings.Cut(req.Host, ","); ok {
				return req, nil
			}
			return req, errors.New("data scheme invalid url")
		}
	}
	if req.Scheme != "http" && req.Scheme != "https" && req.Scheme != "file" && req.Scheme != "view-source" {
		return req, errors.New("unknown scheme")
	}

	if req.Scheme == "file" {
		return req, nil
	}

	req.Host = strings.TrimPrefix(req.Host, "www.")

	req.Host, temp, ok = strings.Cut(req.Host, ":")
	if !ok {
		if req.Scheme == "http" {
			req.Port = 80
		} else {
			req.Port = 443
		}
		req.Host, req.Path, _ = strings.Cut(req.Host, "/")
		req.Path = "/" + req.Path
	} else {
		temp, req.Path, _ = strings.Cut(temp, "/")
		req.Port, err = strconv.Atoi(temp)
		if err != nil {
			return req, err
		}
		req.Path = "/" + req.Path
	}

	req.Add_header("Host", req.Host)
	req.Add_header("Connection", "close")
	req.Add_header("User-Agent", "botted")

	return req, nil
}

func (req Request) Get() (Response, error) {
	switch req.Scheme {
	case "view-source":
		return req.get_view_source()
	case "http", "https":
		return req.get_net()
	case "file":
		return req.get_file()
	case "data":
		return req.get_data()
	default:
		return Response{}, errors.New("unknown scheme")
	}
}

func (req Request) get_view_source() (Response, error) {
	temp_req := req
	temp_req.Scheme = "https"
	temp_resp, err := temp_req.Get()
	temp_resp.Scheme = "view-source"
	return temp_resp, err
}

func (req Request) get_net() (Response, error) {
	resp := Response{}
	resp.Headers = make(map[string]string)
	resp.Scheme = req.Scheme

	var conn net.Conn
	var err error
	url := req.Host + ":" + strconv.Itoa(req.Port)

	if req.Scheme == "https" {
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}
		conn, err = tls.Dial("tcp", url, conf)
	} else {
		conn, err = net.Dial("tcp", url)
	}

	if err != nil {
		return resp, err
	}
	defer conn.Close()

	buf := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	buf.WriteString(req.http_raw())
	buf.Flush()
	resp.read(buf)

	return resp, nil
}

func (req Request) get_file() (Response, error) {
	resp := Response{}
	resp.Headers = make(map[string]string)
	resp.Scheme = req.Scheme

	file, err := os.Open(req.Host)
	if err != nil {
		return resp, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return resp, err
	}
	resp.Status = "HTTP/1.1 200 OK"
	resp.Body = string(content)
	return resp, nil
}

func (req Request) get_data() (Response, error) {
	resp := Response{}
	resp.Headers = make(map[string]string)
	resp.Scheme = req.Scheme
	resp.Status = "HTTP/1.1 200 OK"
	resp.Add_header("Content-Type", req.Host)
	resp.Body = req.Path
	return resp, nil
}

func (req *Request) Add_header(field, value string) {
	if field != "" && value != "" {
		req.Headers[field] = value
	}
}
func (resp *Response) Add_header(field, value string) {
	if field != "" && value != "" {
		resp.Headers[field] = value
	}
}

func (req Request) http_raw() string {
	var ret strings.Builder
	ret.WriteString("GET " + req.Path + " HTTP/1.1\r\n")

	headers := []string{"Host", "User-Agent", "Connection"}
	for _, header := range headers {
		if value, exists := req.Headers[header]; exists {
			ret.WriteString(header + ": " + value + "\r\n")
		}
	}

	for k, v := range req.Headers {
		found := false
		for _, header := range headers {
			if k == header {
				found = true
				break
			}
		}
		if !found {
			ret.WriteString(k + ": " + v + "\r\n")
		}
	}

	ret.WriteString("\r\n")
	return ret.String()
}

func (resp *Response) read(buf *bufio.ReadWriter) {
	statusLine, _, err := buf.ReadLine()
	if err != nil {
		log.Println("error reading status line:", err)
	}
	resp.Status = string(statusLine)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			log.Println("error reading line:", err)
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
			log.Println("error reading body:", err)
		}
		body.Write(line)
		body.WriteString("\n")
	}

	resp.Body = body.String()

	return
}
