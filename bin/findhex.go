package main

import (
	"encoding/hex"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"flag"
	"fmt"
)

var (
	inDir = flag.String("dir", ".", "in dir")
	targetHex = flag.String("s", "", "hex to find")
)

func main() {
	flag.Parse()

	if *targetHex == "" {
		fmt.Println("nothing to find...")
		return
	}

	tbuf, err := hex.DecodeString(*targetHex)
	if err != nil {
		fmt.Println("[err]target Hex error", err)
		return
	}

	filepath.Walk(*inDir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if f.IsDir() {
			return nil
		}

		find(path, tbuf)
		return nil
	})
}

func find(fp string, buf []byte) {
	// open input file
	fd, err := os.OpenFile(fp, os.O_RDWR, 0660)
	if err != nil {
		panic(err)
	}
	// close fd on exit and check for its returned error
	defer func() {
		if err := fd.Close(); err != nil {
			panic(err)
		}
	}()

	// TODO: not read all into RAM
	b, err := ioutil.ReadAll(fd)
	if err != nil {
		fmt.Println("[err]read file err", err)
	}

	seek := 0
	sz := len(buf)
	for {
		idx := bytes.Index(b, buf)
		if idx < 0 {
			return
		}
		fmt.Printf("%v @ 0x%x (%v)\n", fp, seek + idx, seek + idx)

		nextIdx := idx + sz
		if len(b) <= nextIdx {
			return
		}
		seek += nextIdx
		b = b[nextIdx:]
	}


}

