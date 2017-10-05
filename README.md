# tools for myself in golang


## tools

* rpi-upswdt
	* auto poweroff rpi2 when no network
	* test by 'ping' and 'http.Get'
	* wiring: AC power >> zenpower >> rpi2

* network
	* httpd.go : simple static http file server
	* raw2socks.go : proxy a raw tcp connection via a SOCKS5 server
	* socks.go : simple SOCKS5 proxy server
	* httpproxy.go : simple http proxy server
	* jmp.go : raw tcp proxy server

