// This is a simple SOCKS5 proxy server.
// Copyright 2013-2015, physacco. Distributed under the MIT license.

package main

import (
	"io"
//	"os"
//	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
	"runtime"
	"flag"
)

var (
	localAddr = flag.String("l", ":1080", "bind address")

	verbosity = flag.Int("v", 3, "verbosity")
)

// global recycle buffer
var copyBuf sync.Pool

func replyAndClose(p1 net.Conn, rpy int) {
	p1.Write([]byte{0x05, byte(rpy), 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	p1.Close()
}

// thanks: http://www.golangnote.com/topic/141.html
func handleConnection(p1 net.Conn) {
	var b [1024]byte
	n, err := p1.Read(b[:])
	if err != nil {
		Vln(3, "client read", p1, err)
		p1.Close()
		return
	}
	if b[0] != 0x05 { //only Socket5
		p1.Close()
		return
	}

	//reply: NO AUTHENTICATION REQUIRED
	p1.Write([]byte{0x05, 0x00})

	n, err = p1.Read(b[:])
	if b[1] != 0x01 { // 0x01: CONNECT
		replyAndClose(p1, 0x07) // X'07' Command not supported
		return
	}

	var host, port string
	switch b[3] {
	case 0x01: //IP V4
		host = net.IPv4(b[4], b[5], b[6], b[7]).String()
	case 0x03: //DOMAINNAME
		host = string(b[5 : n-2]) //b[4] domain name length
	case 0x04: //IP V6
		host = net.IP{b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15], b[16], b[17], b[18], b[19]}.String()
	default:
		replyAndClose(p1, 0x08) // X'08' Address type not supported
		return
	}
	port = strconv.Itoa(int(b[n-2])<<8 | int(b[n-1]))
	backend := net.JoinHostPort(host, port)
	p2, err := net.DialTimeout("tcp", backend, 5*time.Second)
	if err != nil {
		Vln(2, backend, err)
		replyAndClose(p1, 0x05) // X'05'
		return
	}

	reply := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	Vln(3, "socks to:", backend)
	p1.Write(reply) // reply OK

	go handleClient(p1, p2)
}

func handleClient(p1, p2 io.ReadWriteCloser) {
//	Vln(2, "stream opened")
//	defer Vln(2, "stream closed")
	defer p1.Close()
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

func main() {
	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU() + 2)

	copyBuf.New = func() interface{} {
		return make([]byte, 16384)
	}

	listener, err := net.Listen("tcp", *localAddr)
	if err != nil {
		log.Fatal("Listen error: ", err)
	}
	log.Printf("Listening on %s...\n", *localAddr)


	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func Vf(level int, format string, v ...interface{}) {
	if level <= *verbosity {
		log.Printf(format, v...)
	}
}
func V(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Print(v...)
	}
}
func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}


