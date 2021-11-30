package main

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"
)

var (
	gzipLv = flag.Int("gz", 5, "gzip disable = 0, DefaultCompression = -1, BestSpeed = 1, BestCompression = 9")

	file = flag.String("f", "/:index.html;/index.html:index.html", "allow put file")

	readTimeout  = flag.Int("rt", 5, "http ReadTimeout (Second), <= 0 disable")
	writeTimeout = flag.Int("wt", 0, "http WriteTimeout (Second), <= 0 disable")
	rxSpd        = flag.Int("rx", 1024*1024, "RX speed (byte/sec)")
	txSpd        = flag.Int("tx", 1024*1024, "TX speed (byte/sec)")

	verbosity = flag.Int("v", 3, "verbosity")
	port      = flag.String("l", ":4040", "bind port")
	dir       = flag.String("d", "./www", "bind dir")

	crtFile = flag.String("crt", "", "https certificate file")
	keyFile = flag.String("key", "", "https private key file")
)

func reqlog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Vln(3, r.Method, r.URL, r.RemoteAddr, r.Host)
		if *verbosity >= 6 {
			for i, hdr := range r.Header {
				Vln(6, "---", i, len(hdr), hdr)
			}
			Vln(6, "")
		}
		w.Header().Add("Service-Worker-Allowed", "/")
		gzw := TryGzipResponse(w, r)
		if gzw != nil {
			defer gzw.Close()
			next.ServeHTTP(gzw, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func wiki(next http.Handler) http.Handler {
	allowFp := make(map[string]string)
	urls := strings.Split(*file, ";")
	for _, s := range urls {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) > 1 {
			allowFp[parts[0]] = parts[1]
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "HEAD":
			return
		case "OPTIONS":
			w.Header().Add("Allow", "GET, HEAD, PUT, OPTIONS")
			w.Header().Add("DAV", "1, 2") // hack for WebDAV sync adaptor/saver
			return
		case "PUT":
			fp, ok := allowFp[r.URL.Path]
			if !ok {
				Vln(3, "[put]Forbidden", r.Method, r.URL, r.RemoteAddr, r.Host)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			fp = path.Join(*dir, fp)

			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				Vln(3, "[put]read", r.Method, r.URL, r.RemoteAddr, r.Host, err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			err = ioutil.WriteFile(fp, b, 0644)
			if err != nil {
				Vln(3, "[put]save", r.Method, r.URL, r.RemoteAddr, r.Host, err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			return
		case "GET":
			w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
		default:
		}
		next.ServeHTTP(w, r)
	})
}

type userlist map[string]string   // user -> pass
type AuthDir map[string]*userlist // path -> user list

func basicAuth(w http.ResponseWriter, r *http.Request, h http.Handler, list *userlist) {
	userReq, passReq, _ := r.BasicAuth()
	/*if !ok {
		http.Error(w, "BadRequest", http.StatusBadRequest)
		return
	}*/

	pass, ok := (map[string]string)(*list)[userReq]
	if !ok || pass != passReq {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.ServeHTTP(w, r)
}

func basicAuthDir(h http.Handler, authInfo *AuthDir) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dirList := (map[string]*userlist)(*authInfo)
		for p, list := range dirList {
			if strings.HasPrefix(r.URL.Path, p) {
				basicAuth(w, r, h, list)
				return
			}
		}
		// Vln(3, "HttpAuth Path:", r.URL.Path, ok, list)
		h.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	/*
	if config.HttpAuth != nil {
		Vln(2, "HttpAuth:", config.HttpAuth)
		fileHandler = basicAuthDir(fileHandler, config.HttpAuth)
	}
	*/

	http.Handle("/", reqlog(wiki(http.FileServer(http.Dir(*dir)))))
	srv := &http.Server{
		ReadTimeout:  time.Duration(*readTimeout) * time.Second,
		WriteTimeout: time.Duration(*writeTimeout) * time.Second,
		Addr:         *port,
		Handler:      nil,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("srv -> client (TX) limit: %v\n", *txSpd)
	log.Printf("srv <- client (RX) limit: %v\n", *rxSpd)
	startServer(srv, *crtFile, *keyFile)

	<-idleConnsClosed
}

func startServer(srv *http.Server, crt string, key string) {
	var err error

	// check tls
	if crt != "" && key != "" {
		cfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{

				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,

				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, // http/2 must
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,   // http/2 must

				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,

				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

				tls.TLS_RSA_WITH_AES_256_GCM_SHA384, // weak
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,    // waek
			},
		}
		srv.TLSConfig = cfg
		//srv.TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0) // disable http/2

		log.Printf("[server] HTTPS server Listen on: %v", srv.Addr)
		err = srv.ListenAndServeTLS(crt, key)
	} else {
		log.Printf("[server] HTTP server Listen on: %v", srv.Addr)
		err = srv.ListenAndServe()
	}

	if err != http.ErrServerClosed {
		log.Printf("[server] ListenAndServe error: %v", err)
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

type GzipResponseWriter struct {
	http.ResponseWriter
	gzip *gzip.Writer
}

func (w *GzipResponseWriter) Write(p []byte) (int, error) {
	if w.gzip == nil {
		return w.ResponseWriter.Write(p)
	}

	return w.gzip.Write(p)
}

func (w *GzipResponseWriter) Close() error {
	if w.gzip != nil {
		return w.gzip.Close()
	}
	return nil
}

func CanAcceptsGzip(r *http.Request) bool {
	s := strings.ToLower(r.Header.Get("Accept-Encoding"))
	for _, ss := range strings.Split(s, ",") {
		if strings.HasPrefix(ss, "gzip") {
			return true
		}
	}
	return false
}

func TryGzipResponse(w http.ResponseWriter, r *http.Request) *GzipResponseWriter {
	if !CanAcceptsGzip(r) || *gzipLv == 0 {
		return nil
	}

	gw, err := gzip.NewWriterLevel(w, *gzipLv)
	if err != nil {
		gw = gzip.NewWriter(w)
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")

	return &GzipResponseWriter{w, gw}
}

type SpeedCtrl struct {
	In net.Conn
	Tx int64
	Rx int64

	die     chan struct{}
	dieLock sync.Mutex

	rxLim float64
	rx0   int64
	rxt   time.Time

	txLim float64
	tx0   int64
	txt   time.Time
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

func (c *SpeedCtrl) Read(data []byte) (n int, err error) {
	n, err = c.In.Read(data)
	curr := atomic.AddInt64(&c.Rx, int64(n))

	if c.rxLim <= 0 {
		return
	}

	now := time.Now()
	emsRx := int64(c.rxLim*now.Sub(c.rxt).Seconds()) + c.rx0
	if curr > emsRx {
		over := curr - emsRx
		sleep := float64(over) / c.rxLim
		sleepT := time.Duration(sleep*1000000000) * time.Nanosecond
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
	emsTx := int64(c.txLim*now.Sub(c.txt).Seconds()) + c.tx0
	if curr > emsTx {
		over := curr - emsTx
		sleep := float64(over) / c.txLim
		sleepT := time.Duration(sleep*1000000000) * time.Nanosecond
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
