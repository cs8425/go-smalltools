// proxy a raw tcp connection via a socks5 proxy
// raw tcp client ---> socks5 proxy ---> raw tcp server
package main

import (
	"io"
	"os"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
	"runtime"
)

var LISTEN string  // listen address, e.g. 0.0.0.0:1080
var SOCKS_SERVER string  // socks5_server address, e.g. 123.123.123.123:1080
var TARGET string  // target address, e.g. www.google.com:80

var verbosity int = 3

// global recycle buffer
var copyBuf sync.Pool

var socksReq []byte

func handleConnection(p1 net.Conn) {
	defer p1.Close()

	p2, err := net.DialTimeout("tcp", SOCKS_SERVER, 5*time.Second)
	if err != nil {
		Vlogln(2, "connect to ", SOCKS_SERVER, err)
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
		Vlogln(2, "Dial err replay:", SOCKS_SERVER, n)
		return
	}
	if err != nil || b[1] != 0x00 {
		Vlogln(2, "Dial err:", SOCKS_SERVER, n, b[1], err)
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
    if len(os.Args) < 4 {
        fmt.Println("Usage: raw2socks5 LISTEN SOCKS_SERVER TARGET")
        return
    }

    LISTEN = os.Args[1]
	SOCKS_SERVER =  os.Args[2]
	TARGET =  os.Args[3]

	host, portStr, err := net.SplitHostPort(TARGET)
	if err != nil {
		Vlogln(2, "SplitHostPort err:", TARGET, err)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		Vlogln(2, "failed to parse port number:", portStr, err)
		return
	}
	if port < 1 || port > 0xffff {
		Vlogln(2, "port number out of range:", portStr, err)
		return
	}

	socksReq = []byte{0x05, 0x01, 0x00, 0x03}
	socksReq = append(socksReq, byte(len(host)))
	socksReq = append(socksReq, host...)
	socksReq = append(socksReq, byte(port>>8), byte(port))

	runtime.GOMAXPROCS(runtime.NumCPU() + 2)
	copyBuf.New = func() interface{} {
		return make([]byte, 4096)
	}

	listener, err := net.Listen("tcp", LISTEN)
	if err != nil {
		log.Fatal("Listen error: ", err)
	}
	log.Printf("Listening on %s...\n", LISTEN)


	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go handleConnection(conn)
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

