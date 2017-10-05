// proxy a raw tcp connection via a socks5 proxy
// raw tcp client ---> socks5 proxy ---> raw tcp server
package main

import (
	"io"
	"flag"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
	"runtime"
)

var verbosity = flag.Int("v", 3, "verbosity")

var localAddr = flag.String("l", ":9999", "bind addr")
var socksAddr = flag.String("s", "example.com:1080", "socks5 server addr")
var targetAddr = flag.String("t", "192.168.1.1:80", "target addr")

// global recycle buffer
var copyBuf sync.Pool

var socksReq []byte

func handleConnection(p1 net.Conn) {
	defer p1.Close()

	p2, err := net.DialTimeout("tcp", *socksAddr, 5*time.Second)
	if err != nil {
		Vln(2, "connect to ", *socksAddr, err)
		return
	}

	var b [10]byte

	// send request
	p2.Write([]byte{0x05, 0x01, 0x00})

	// read reply
	_, err = p2.Read(b[:2])
	if err != nil {
		return
	}

	// send server addr
	p2.Write(socksReq)

	// read reply
	n, err := p2.Read(b[:10])
	if n < 10 {
		Vln(2, "Dial err replay:", *socksAddr, n)
		return
	}
	if err != nil || b[1] != 0x00 {
		Vln(2, "Dial err:", *socksAddr, n, b[1], err)
		return
	}

	cp(p1, p2)
}

func cp(p1, p2 io.ReadWriteCloser) {
//	defer p1.Close()
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
		return make([]byte, 4096)
	}

	host, portStr, err := net.SplitHostPort(*targetAddr)
	if err != nil {
		Vln(2, "SplitHostPort err:", *targetAddr, err)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		Vln(2, "failed to parse port number:", portStr, err)
		return
	}
	if port < 1 || port > 0xffff {
		Vln(2, "port number out of range:", portStr, err)
		return
	}

	socksReq = []byte{0x05, 0x01, 0x00, 0x03}
	socksReq = append(socksReq, byte(len(host)))
	socksReq = append(socksReq, host...)
	socksReq = append(socksReq, byte(port>>8), byte(port))


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

