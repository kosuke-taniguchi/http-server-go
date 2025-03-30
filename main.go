package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

var _ = net.Listen
var _ = os.Exit

var (
	contentTypeTextPlain   = "text/plain"
	contentTypeOctetStream = "application/octet-stream"
)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go serve(conn)
	}
}

func serve(conn net.Conn) {
	defer conn.Close()
	req := make([]byte, 1024)
	_, err := conn.Read(req)
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		os.Exit(1)
	}
	request := newRequest(conn, req)
	if request == nil {
		request.badRequest()
		return
	}
	switch request.method {
	case "GET":
		request.handleGet()
	default:
		request.notFound()
	}
}

type request struct {
	conn    net.Conn
	method  string
	path    string
	headers map[string]string
	body    []byte
}

func newRequest(conn net.Conn, req []byte) *request {
	scanner := bufio.NewScanner(bytes.NewReader(req))
	scanner.Scan()
	// request line
	reqLine := strings.Split(scanner.Text(), " ")
	if len(reqLine) < 3 {
		return nil
	}
	// headers
	headers := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		} // header終了
		parts := strings.SplitN(line, ":", 2)
		headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	// body
	var body bytes.Buffer
	for scanner.Scan() {
		body.Write(scanner.Bytes())
	}
	return &request{
		conn:    conn,
		method:  reqLine[0],
		path:    reqLine[1],
		headers: headers,
		body:    body.Bytes(),
	}
}

func (r *request) handleGet() {
	endpoint := strings.Split(r.path, "/")[1]
	switch endpoint {
	case "":
		r.ok(contentTypeTextPlain, []byte(""))
		return
	case "echo":
		r.getEcho()
	case "user-agent":
		r.getUseragent()
	case "files":
		r.getFiles()
	default:
		r.notFound()
	}
}

func (r *request) badRequest() {
	r.conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n"))
}

func (r *request) notFound() {
	r.conn.Write([]byte("HTTP/1.1 404 Not Found\r\n"))
}

func (r *request) ok(contentType string, body []byte) {
	r.conn.Write([]byte(fmt.Sprintf(
		"HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
		contentType,
		len(body),
		body,
	)))
}
func (r *request) getEcho() {
	pathParam := strings.TrimPrefix(r.path, "/echo/")
	r.ok(contentTypeTextPlain, []byte(pathParam))
}

func (r *request) getUseragent() {
	ua, ok := r.headers["User-Agent"]
	if !ok {
		r.badRequest()
		return
	}
	r.ok(contentTypeTextPlain, []byte(ua))
}

func (r *request) getFiles() {
	filename := strings.TrimPrefix(r.path, "/files/")
	fp, err := os.Open(fmt.Sprintf("/tmp/%s", filename))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		log.Println("file not found: ", filename)
		r.notFound()
		return
	}
	if err != nil {
		r.badRequest()
		return
	}
	defer fp.Close()
	body, err := io.ReadAll(fp)
	if err != nil {
		r.badRequest()
		return
	}
	r.ok(contentTypeOctetStream, body)
}
