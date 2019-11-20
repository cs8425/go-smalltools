package main

import (
	"compress/gzip"
	"net/http"
	"strings"
	"flag"
	"log"
)

var (
	gzipLv = flag.Int("gz", 5, "gzip disable = 0, DefaultCompression = -1, BestSpeed = 1, BestCompression = 9")

	verbosity = flag.Int("v", 3, "verbosity")
	port = flag.String("l", ":4040", "bind port")
	dir = flag.String("d", "./www", "bind dir")
)

func reqlog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Vln(3, r.Method, r.URL, r.RemoteAddr)
		gzw := TryGzipResponse(w, r)
		defer gzw.Close()
		next.ServeHTTP(gzw, r)
	})
}

func main() {
	flag.Parse()
	http.Handle("/", reqlog(http.FileServer(http.Dir(*dir))))
	err := http.ListenAndServe(*port, nil)
	if err != nil {
		Vln(0, err)
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

func (w *GzipResponseWriter) Close() (error) {
	if w.gzip != nil {
		return w.gzip.Close()
	}
	return nil
}

func CanAcceptsGzip(r *http.Request) (bool) {
	s := strings.ToLower(r.Header.Get("Accept-Encoding"))
	for _, ss := range strings.Split(s, ",") {
		if strings.HasPrefix(ss, "gzip") {
			return true
		}
	}
	return false
}

func TryGzipResponse(w http.ResponseWriter, r *http.Request) (*GzipResponseWriter) {
	if !CanAcceptsGzip(r) || *gzipLv == 0 {
		return &GzipResponseWriter{w, nil}
	}

	gw, err := gzip.NewWriterLevel(w, *gzipLv)
	if err != nil {
		gw = gzip.NewWriter(w)
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")

	return &GzipResponseWriter{w, gw}
}

