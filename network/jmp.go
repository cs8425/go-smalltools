package main

import (
	"io"
	"log"
//	"crypto/tls"
	"net"
	"flag"
	"sync"
	"runtime"
)

var localAddr = flag.String("from", ":9999", "")
var remoteAddr = flag.String("to", "127.0.0.1:80", "")

// global recycle buffer
var copyBuf sync.Pool

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	copyBuf.New = func() interface{} {
		return make([]byte, 4096)
	}

	/*cer, err := tls.LoadX509KeyPair("server.pem", "server.key")
	if err != nil {
		log.Println(err)
		return
	}

	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	ln, err := tls.Listen("tcp", *localAddr, config) */
	addr, err := net.ResolveTCPAddr("tcp", *localAddr)
	if err != nil {
		panic(err)
	}
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Println(err)
		return
	}
	defer ln.Close()

	log.Printf("Listening: %v -> %v\n\n", *localAddr, *remoteAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go proxyConn(conn)
	}
}

func proxyConn(conn net.Conn) {
	defer conn.Close()

	rAddr, err := net.ResolveTCPAddr("tcp", *remoteAddr)
	if err != nil {
		log.Print(err)
	}

	rConn, err := net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		log.Print(err)
		return
	}
	defer rConn.Close()

	cp(conn, rConn)

//	log.Printf("handleConnection end: %s\n", conn.RemoteAddr())
}

func cp(p1, p2 io.ReadWriteCloser) {
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


