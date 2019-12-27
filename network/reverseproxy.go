/*
simple https to http proxy server
for testing PWA on local side

Create Root Key
openssl genrsa -des3 -out rootCA.key 4096
openssl ecparam -genkey -name secp384r1 -out rootCA.key

Create and self sign the Root Certificate
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 3650 -out rootCA.crt

Create the signing (csr)
openssl req -new -sha256 -key server.key -subj "/C=TW/O=OAC/CN=lvh.me" -out server.csr


Generate the certificate using the mydomain csr and key along with the CA Root key
openssl x509 -req -in server.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out server.crt -days 3650 -sha256
openssl x509 -req \
        -extfile <(printf "[v3_req]\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:*.lvh.me,DNS:lvh.me") \
        -extensions v3_req \
        -days 3650 -in server.csr -CA rootCA.crt -CAkey rootCA.key \
        -CAcreateserial -out server.crt -sha256

*/
package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
	"flag"
	"log"
)

var (
	port = flag.String("p", "127.0.0.1:8080", "https bind port")
	target = flag.String("t", "http://127.0.0.1:4040/", "target url")

	crtFile    = flag.String("crt", "cert/server.crt", "PEM encoded certificate file")
	keyFile    = flag.String("key", "cert/server.key", "PEM encoded private key file")
)

func main() {
	flag.Parse()

	//http://127.0.0.1:4040/
	u, _ := url.Parse(*target)
	proxy := httputil.NewSingleHostReverseProxy(u)
	dir0 := proxy.Director
	dir := func(req *http.Request) {
		dir0(req)
		req.Host = u.Host
	}
	proxy.Director = dir
	http.Handle("/", proxy)

	// start http server
	srv := &http.Server{
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr: *port,
		Handler: nil,
	}
	startServer(srv)
}

func startServer(srv *http.Server) {
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
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, // http/2 must

			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,

			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

			tls.TLS_RSA_WITH_AES_256_GCM_SHA384, // weak
			tls.TLS_RSA_WITH_AES_256_CBC_SHA, // waek
		},
	}
	srv.TLSConfig = cfg
	//srv.TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0) // disable http/2

	log.Printf("HTTPS server Listen on: %v", *port)
	err := srv.ListenAndServeTLS(*crtFile, *keyFile)

	if err != http.ErrServerClosed {
		log.Printf("ListenAndServe error: %v", err)
	}
}

