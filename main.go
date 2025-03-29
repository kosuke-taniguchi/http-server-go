package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
)

var _ = net.Listen
var _ = os.Exit

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	conn, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}
	defer conn.Close()
	req := make([]byte, 1024)
	_, err = conn.Read(req)
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		os.Exit(1)
	}
	split := bytes.Split(req, []byte("\r\n"))
	if len(split) < 2 {
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n"))
		return
	}
	reqLine := split[0]
	split = bytes.Split(reqLine, []byte(" "))
	if len(split) < 3 {
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n"))
		return
	}
	method, path := string(split[0]), string(split[1])
	if method == "GET" && path == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
		return
	}
	conn.Write([]byte("HTTP/1.1 404 Not Found\r\n"))
}
