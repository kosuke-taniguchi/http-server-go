package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
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
	// ContentType
	contentTypeTextPlain   = "text/plain"
	contentTypeOctetStream = "application/octet-stream"
	// headers
	headerUserAgent   = "User-Agent"
	headerContentType = "Content-Type"
)

func main() {
	var directory string
	flag.StringVar(&directory, "directory", "/tmp", "directory from which to serve files")
	flag.Parse()
	info, err := os.Stat(directory)
	if err != nil {
		log.Println("Failed to stat directory: ", err.Error())
		os.Exit(1)
	}
	if !info.IsDir() {
		log.Println("Directory does not exist: ", directory)
		os.Exit(1)
	}
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		log.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go serve(conn, directory)
	}
}

func serve(conn net.Conn, dir string) {
	defer conn.Close()
	req := make([]byte, 1024)
	_, err := conn.Read(req)
	if err != nil {
		log.Println("Error reading request: ", err.Error())
		os.Exit(1)
	}
	request := newRequest(conn, req, dir)
	if request == nil {
		request.badRequest()
		return
	}
	request.routes()
}

type request struct {
	conn    net.Conn
	method  string
	path    string
	headers map[string]string
	body    []byte
	fileDir string
}

func newRequest(conn net.Conn, req []byte, dir string) *request {
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
		fileDir: dir,
	}
}

func (r *request) routes() {
	endpoint := strings.Split(r.path, "/")[1]
	switch endpoint {
	case "echo":
		switch r.method {
		case "GET":
			r.getEcho()
		default:
			r.notFound()
		}
	case "user-agent":
		switch r.method {
		case "GET":
			r.getUseragent()
		default:
			r.notFound()
		}
	case "files":
		switch r.method {
		case "GET":
			r.getFiles()
		case "POST":
			r.postFiles()
		default:
			r.notFound()
		}
	case "":
		switch r.method {
		case "GET":
			r.ok(contentTypeTextPlain, []byte(""))
		default:
			r.notFound()
		}
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

func (r *request) created() {
	r.conn.Write([]byte(fmt.Sprintf("HTTP/1.1 201 Created\r\n")))
}

func (r *request) getEcho() {
	pathParam := strings.TrimPrefix(r.path, "/echo/")
	r.ok(contentTypeTextPlain, []byte(pathParam))
}

func (r *request) getUseragent() {
	ua, ok := r.headers[headerUserAgent]
	if !ok {
		r.badRequest()
		return
	}
	r.ok(contentTypeTextPlain, []byte(ua))
}

func (r *request) getFiles() {
	filename := strings.TrimPrefix(r.path, "/files/")
	fp, err := os.Open(fmt.Sprintf("%s/%s", r.fileDir, filename))
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

func (r *request) postFiles() {
	if r.headers["Content-Type"] != contentTypeOctetStream {
		r.badRequest()
		return
	}
	filename := strings.TrimPrefix(r.path, "/files/")
	fp, err := os.Create(fmt.Sprintf("%s/%s", r.fileDir, filename))
	if err != nil {
		r.badRequest()
		return
	}
	defer fp.Close()
	_, err = fp.Write(r.body)
	if err != nil {
		r.badRequest()
		return
	}
	r.created()
}
