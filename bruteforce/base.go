package main

import (
	"crypto/sha256"

	"encoding/hex"
	"bytes"

	"sync"
	"runtime"

//	"fmt"
	"log"
	"flag"
)

var bs = flag.Int("s", 2, "start")
var be = flag.Int("e", 4, "end")
var n = flag.Int("n", 8, "cpu")

// sha256("00")
var target = flag.String("target", "f1534392279bddbf9d43dde8701cb5be14b82f76ec6607bf8d6ad557f60f304e", "sha256 to try, default=sha256(\"00\")")

var verbosity int = 2

func main() {
	flag.Parse()

	num := *n
//	num := runtime.NumCPU()
	runtime.GOMAXPROCS(num + 1)

	var err error
	hash, err = hex.DecodeString(*target)
	if err != nil {
		Vln(0, "target no a hex string", )
		return
	}

	Vln(1, "[cpu]", num)
	Vln(1, "[start]", *bs)
	Vln(1, "[end]", *be)
	Vln(1, "[target]", *target)

	var wg sync.WaitGroup

	ch := make(chan []byte, num)
	end := make(chan struct{}, 1)

//	i := runtime.NumCPU()
	i := num
	for i > 0 {
		wg.Add(1)
		go worker(i, ch, end, &wg)

		i--
	}


	start := *bs
	input := make([]byte, start)
	for start <= *be {
		tmp := make([]byte, start)
		copy(tmp, input)
		select {
		case <-end:
			goto WAIT_END
		case ch <- tmp:
			if increment(input) {
				start++
				input = make([]byte, start)
				Vln(2, "go next!", start)
			}
		//default:
		}
	}

WAIT_END:
	endCh(end)

	wg.Wait()
	Vln(2, "Done!")
}

func worker(id int, in chan []byte, end chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	var job []byte

	for {
		select {
		case <-end:
			return
		case job = <-in:
		}

		if calcOk(job) {
			Vln(2, "[GJ]", id, hex.EncodeToString( job ), string(job))
			endCh(end)
			return
		}
	}
}

func endCh(end chan struct{}) {
	select {
	case <-end:
	default:
		close(end)
	}
}

func Vf(level int, format string, v ...interface{}) {
	if level <= verbosity {
		log.Printf(format, v...)
	}
}
func V(level int, v ...interface{}) {
	if level <= verbosity {
		log.Print(v...)
	}
}
func Vln(level int, v ...interface{}) {
	if level <= verbosity {
		log.Println(v...)
	}
}

// gen next key
func increment(b []byte) (bool) {
	for i := range b {
		b[i]++
		if b[i] != 0 {
			return false
		}
	}
	return true
}


// real check
func calcOk(in []byte) (bool){
	h := HashBytes256(in)
	return bytes.Equal(h, hash)
}

var hash []byte
func HashBytes256(a []byte) []byte {
	sha1h := sha256.New()
	sha1h.Write(a)
	return sha1h.Sum([]byte(""))
}

