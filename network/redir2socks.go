// proxy any tcp connection from iptables REDIRECT to socks5 proxy
// raw tcp client ---> iptables REDIRECT ---> socks5 proxy ---> raw tcp server
package main

import (
	"io"
	"flag"
	"log"
	"net"
	"sync"
	"time"
	"runtime"

	"unsafe"
	"syscall"
	"errors"

//	"fmt"
	"strconv"
)

var verbosity = flag.Int("v", 3, "verbosity")

var localAddr = flag.String("l", ":7777", "bind addr")
var socksAddr = flag.String("s", "example.com:1080", "socks5 server addr")
var targetAddr = flag.String("t", "192.168.1.1:80", "target addr")

// global recycle buffer
var copyBuf sync.Pool


// thank's https://github.com/shadowsocks/go-shadowsocks2
const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4
)

type Addr []byte

// String serializes SOCKS address a to string form.
func (a Addr) String() string {
	var host, port string

	switch a[0] { // address type
	case AtypDomainName:
		host = string(a[2 : 2+int(a[1])])
		port = strconv.Itoa((int(a[2+int(a[1])]) << 8) | int(a[2+int(a[1])+1]))
	case AtypIPv4:
		host = net.IP(a[1 : 1+net.IPv4len]).String()
		port = strconv.Itoa((int(a[1+net.IPv4len]) << 8) | int(a[1+net.IPv4len+1]))
	case AtypIPv6:
		host = net.IP(a[1 : 1+net.IPv6len]).String()
		port = strconv.Itoa((int(a[1+net.IPv6len]) << 8) | int(a[1+net.IPv6len+1]))
	}

	return net.JoinHostPort(host, port)
}

const (
	SO_ORIGINAL_DST      = 80 // from linux/include/uapi/linux/netfilter_ipv4.h
	IP6T_SO_ORIGINAL_DST = 80 // from linux/include/uapi/linux/netfilter_ipv6/ip6_tables.h
)

// Get the original destination of a TCP connection.
func getOrigDst(conn net.Conn) (Addr, error) {
	c, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, errors.New("only work with TCP connection")
	}
	f, err := c.File()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fd := f.Fd()

	// The File() call above puts both the original socket fd and the file fd in blocking mode.
	// Set the file fd back to non-blocking mode and the original socket fd will become non-blocking as well.
	// Otherwise blocking I/O will waste OS threads.
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		return nil, err
	}

	addr, err := ipv6_getorigdst(fd)
	Vln(3, "ipv6_getorigdst:", addr, err)
	if err != nil {
		return getorigdst(fd)
	}

	return addr, nil
}

// Call getorigdst() from linux/net/ipv4/netfilter/nf_conntrack_l3proto_ipv4.c
func getorigdst(fd uintptr) (Addr, error) {
	raw := syscall.RawSockaddrInet4{}
	siz := unsafe.Sizeof(raw)
	if _, _, err := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.IPPROTO_IP, SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&siz)), 0); err != 0 {
		return nil, err
	}

	addr := make([]byte, 1+net.IPv4len+2)
	addr[0] = AtypIPv4
	copy(addr[1:1+net.IPv4len], raw.Addr[:])
	port := (*[2]byte)(unsafe.Pointer(&raw.Port)) // big-endian
	addr[1+net.IPv4len], addr[1+net.IPv4len+1] = port[0], port[1]
	return addr, nil
}

// Call ipv6_getorigdst() from linux/net/ipv6/netfilter/nf_conntrack_l3proto_ipv6.c
// NOTE: I haven't tried yet but it should work since Linux 3.8.
func ipv6_getorigdst(fd uintptr) (Addr, error) {
	raw := syscall.RawSockaddrInet6{}
	siz := unsafe.Sizeof(raw)
	if _, _, err := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.IPPROTO_IPV6, IP6T_SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&siz)), 0); err != 0 {
//	if _, _, err := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.SOL_IPV6, IP6T_SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&siz)), 0); err != 0 {
		return nil, err
	}

	addr := make([]byte, 1+net.IPv6len+2)
	addr[0] = AtypIPv6
	copy(addr[1:1+net.IPv6len], raw.Addr[:])
	port := (*[2]byte)(unsafe.Pointer(&raw.Port)) // big-endian
	addr[1+net.IPv6len], addr[1+net.IPv6len+1] = port[0], port[1]
	return addr, nil
}

func main() {
	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU() + 2)

	copyBuf.New = func() interface{} {
		return make([]byte, 4096)
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

func handleConnection(p1 net.Conn) {
	defer p1.Close()

	addr, err := getOrigDst(p1)
	if err != nil {
		Vln(2, "err get getOrigDst", p1, err)
		return
	}
	Vln(2, "OriginalDst:", addr, p1)

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

	socksReq := []byte{0x05, 0x01, 0x00}
	socksReq = append(socksReq, addr...)
	// send server addr
	p2.Write(socksReq)

	// read reply
	n, err := p2.Read(b[:10])
	if n < 10 {
		Vln(2, "Dial err replay:", addr, n)
		return
	}
	if err != nil || b[1] != 0x00 {
		Vln(2, "Dial err:", addr, n, b[1], err)
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

