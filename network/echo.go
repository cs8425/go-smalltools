package main

import (
	"flag"
	"io"
	"log"
	"net"
	"runtime"
	"sync"
)

var localAddr = flag.String("l", ":1082", "")

// global recycle buffer
var copyBuf sync.Pool

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	copyBuf.New = func() interface{} {
		return make([]byte, 4096)
	}

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

	log.Printf("Listening: %v\n\n", *localAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go echoConn(conn)
	}
}

func echoConn(conn net.Conn) {
	log.Printf("Connection start: %s\n", conn.RemoteAddr())
	defer conn.Close()

	cp(conn, conn)

	log.Printf("Connection end: %s\n", conn.RemoteAddr())
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
