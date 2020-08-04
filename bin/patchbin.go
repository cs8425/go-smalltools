package main

import (
	"encoding/hex"
//	"io"
	"os"
	"flag"
	"fmt"
)

var file = flag.String("f", "", "file to patch")
var wHex = flag.String("w", "", "hex seq to write")
var num = flag.Int("n", 0, "Byte to read/write")
var seek = flag.Int64("seek", 0, "Byte to seek")
var base = flag.Int64("base", 0, "address base")

func main() {
	flag.Parse()

	fmt.Println("file:", *file)
	fmt.Println("w:", *wHex)
	fmt.Println("n:", *num)
	fmt.Printf("seek: 0x%x\n", *seek)
	fmt.Printf("base: 0x%x\n", *base)

	rseek := *base + *seek


	wbuf, err := hex.DecodeString(*wHex)
	if err != nil {
		fmt.Println("ERROR: wHex error", err)
		return
	}
	wlen := len(wbuf)
	rlen := wlen
	if wlen == 0 {
		rlen =*num
	}

	// open input file
	fi, err := os.OpenFile(*file, os.O_RDWR, 0660)
	if err != nil {
		panic(err)
	}
	// close fi on exit and check for its returned error
	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()

	// make a buffer to keep chunks that are read
	buf := make([]byte, rlen)
	offset, err := fi.Seek(rseek, 0)
	if err != nil {
		panic(err)
	}
	n, err := fi.Read(buf)
	if err != nil {
		fmt.Println("ERROR Read!")
		panic(err)
	}
	fmt.Printf("Read %d bytes @ 0x%x:\n", n, offset)
	fmt.Printf("%v: %02x\n", rlen, buf)

	// nothing to write, return
	if wlen <= 0 {
		return
	}

	// show bytes to write
	fmt.Printf("%v: %02x\n", wlen, wbuf)

	// seek again
	offset, err = fi.Seek(rseek, 0)
	if err != nil {
		panic(err)
	}
	n, err = fi.Write(wbuf)
	fmt.Printf("Write %d bytes @ 0x%x:\n", n, offset)
	if err != nil {
		fmt.Println("ERROR Write!")
		panic(err)
	}
}
