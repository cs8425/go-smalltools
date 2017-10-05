package main

import (
    "fmt"
    "net/http"
	"flag"
)

var port = flag.String("p", ":4040", "bind port")
var dir = flag.String("d", "./", "bind dir")
func main() {
	flag.Parse()
	http.Handle("/", http.FileServer(http.Dir(*dir)))
	err := http.ListenAndServe(*port, nil)
	if err != nil {
		fmt.Println(err)
	}
}

