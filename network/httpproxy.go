package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"runtime"
	"strings"
	"sync"
)

var (
	verbosity = 3
	port      = flag.String("l", ":4040", "bind port")

	// global recycle buffer
	copyBuf = sync.Pool{
		New: func() interface{} {
			return make([]byte, 16384)
		},
	}
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	runtime.GOMAXPROCS(runtime.NumCPU() + 2)

	listener, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatal("Listen error: ", err)
	}
	log.Printf("Listening on %s...\n", *port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go handleClientRequest(conn)
	}

}

// thanks: http://www.golangnote.com/topic/141.html
func handleClientRequest(client net.Conn) {
	defer client.Close()

	var b [1024]byte
	n, err := client.Read(b[:])
	if err != nil {
		Vlogln(3, "client read err", client, err)
		return
	}
	var method, host, address string
	fmt.Sscanf(string(b[:bytes.IndexByte(b[:], '\n')]), "%s%s", &method, &host)

	if strings.Index(host, "://") == -1 {
		host = "//" + host
	}
	hostPortURL, err := url.Parse(host)
	if err != nil {
		Vlogln(3, "Parse hostPortURL err:", client, hostPortURL, err)
		return
	}
	if strings.Index(hostPortURL.Host, ":") == -1 { // no port, default 80
		address = hostPortURL.Host + ":80"
	} else {
		address = hostPortURL.Host
	}

	Vlogln(3, "Dial to:", method, address)
	server, err := net.Dial("tcp", address)
	if err != nil {
		Vlogln(2, "Dial err:", address, err)
		return
	}
	if method == "CONNECT" {
		client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	} else {
		server.Write(b[:n])
	}

	handleClient(client, server)
}

func handleClient(p1, p2 io.ReadWriteCloser) {
	// defer p1.Close()
	defer p2.Close()

	// start tunnel
	p1die := make(chan struct{})
	go func() {
		buf := copyBuf.Get().([]byte)
		io.CopyBuffer(p1, p2, buf)
		close(p1die)
		copyBuf.Put(buf)
	}()

	p2die := make(chan struct{})
	go func() {
		buf := copyBuf.Get().([]byte)
		io.CopyBuffer(p2, p1, buf)
		close(p2die)
		copyBuf.Put(buf)
	}()

	// wait for tunnel termination
	select {
	case <-p1die:
	case <-p2die:
	}
}

func Vlogf(level int, format string, v ...interface{}) {
	if level <= verbosity {
		log.Printf(format, v...)
	}
}
func Vlog(level int, v ...interface{}) {
	if level <= verbosity {
		log.Print(v...)
	}
}
func Vlogln(level int, v ...interface{}) {
	if level <= verbosity {
		log.Println(v...)
	}
}
