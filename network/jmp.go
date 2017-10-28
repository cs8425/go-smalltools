package main

import (
	"io"
	"log"
//	"crypto/tls"
	"net"
	"flag"
	"sync"
	"sync/atomic"
	"runtime"
	"time"
)

var localAddr = flag.String("from", ":9999", "")
var remoteAddr = flag.String("to", "127.0.0.1:80", "")

var RxSpd = flag.Int("rx", 1024*1024, "RX speed (byte/sec)")
var TxSpd = flag.Int("tx", 1024*1024, "TX speed (byte/sec)")

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

	log.Printf("jmp -> client (TX) limit: %v\n", *TxSpd)
	log.Printf("jmp <- client (RX) limit: %v\n", *RxSpd)
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

	var p1 net.Conn = conn

	if *RxSpd > 0 || *TxSpd > 0 {
		spdlim := NewSpeedCtrl(p1)
		p1 = spdlim
		if *RxSpd > 0 {
			spdlim.SetRxSpd(*RxSpd)
		}
		if *TxSpd > 0 {
			spdlim.SetTxSpd(*TxSpd)
		}
	}

	cp(p1, rConn)

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

type SpeedCtrl struct {
	In           net.Conn
	Tx           int64
	Rx           int64

	die          chan struct{}
	dieLock      sync.Mutex


	rxLim        float64
	rx0          int64
	rxt          time.Time

	txLim        float64
	tx0          int64
	txt          time.Time
}

func (c *SpeedCtrl) Close() error {
	c.dieLock.Lock()

	select {
	case <-c.die:
		c.dieLock.Unlock()
		return nil
	default:
	}

	close(c.die)
	return c.In.Close()
}

func (c *SpeedCtrl) Read(data []byte) (n int, err error)  {
	n, err = c.In.Read(data)
	curr := atomic.AddInt64(&c.Rx, int64(n))

	if c.rxLim <= 0 {
		return
	}

	now := time.Now()
	emsRx := int64(c.rxLim * now.Sub(c.rxt).Seconds()) + c.rx0
	if curr > emsRx {
		over := curr - emsRx
		sleep := float64(over) / c.rxLim
		sleepT := time.Duration(sleep * 1000000000) * time.Nanosecond
//log.Println("[Rx over]", curr, emsRx, over, sleepT)
		select {
		case <-c.die:
			return n, err
		case <-time.After(sleepT):
		}
	} else {
		c.rxt = now
		c.rx0 = curr
	}

	return n, err
}

func (c *SpeedCtrl) Write(data []byte) (n int, err error) {
	n, err = c.In.Write(data)
	curr := atomic.AddInt64(&c.Tx, int64(n))

	if c.txLim <= 0 {
		return
	}

	now := time.Now()
	emsTx := int64(c.txLim * now.Sub(c.txt).Seconds()) + c.tx0
	if curr > emsTx {
		over := curr - emsTx
		sleep := float64(over) / c.txLim
		sleepT := time.Duration(sleep * 1000000000) * time.Nanosecond
//log.Println("[Tx over]", curr, emsTx, over, sleepT)
		select {
		case <-c.die:
			return n, err
		case <-time.After(sleepT):
		}
	} else {
		c.txt = now
		c.tx0 = curr
	}

	return n, err
}

// LocalAddr satisfies net.Conn interface
func (c *SpeedCtrl) LocalAddr() net.Addr {
	if ts, ok := c.In.(interface {
		LocalAddr() net.Addr
	}); ok {
		return ts.LocalAddr()
	}
	return nil
}

// RemoteAddr satisfies net.Conn interface
func (c *SpeedCtrl) RemoteAddr() net.Addr {
	if ts, ok := c.In.(interface {
		RemoteAddr() net.Addr
	}); ok {
		return ts.RemoteAddr()
	}
	return nil
}

func (c *SpeedCtrl) SetReadDeadline(t time.Time) error {
	return c.In.SetReadDeadline(t)
}

func (c *SpeedCtrl) SetWriteDeadline(t time.Time) error {
	return c.In.SetWriteDeadline(t)
}

func (c *SpeedCtrl) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func NewSpeedCtrl(con net.Conn) (c *SpeedCtrl) {
	c = &SpeedCtrl{}
	c.die = make(chan struct{})
	c.In = con

	now := time.Now()
	c.rxt = now
	c.txt = now

	return c
}

// Bytes / sec
func (c *SpeedCtrl) SetRxSpd(spd int) {
	now := time.Now()
	c.rxt = now
	c.rx0 = c.Rx
	c.rxLim = float64(spd)
}

func (c *SpeedCtrl) SetTxSpd(spd int) {
	now := time.Now()
	c.txt = now
	c.tx0 = c.Tx
	c.txLim = float64(spd)
}

