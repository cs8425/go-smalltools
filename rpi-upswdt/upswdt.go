package main

import (
//	"net"
	"net/http"
	"net/url"
	"errors"
	"os/exec"
	"strings"

	"flag"
	"log"
	"time"
//	"runtime"
)

var (
	verbosity = flag.Int("v", 2, "verbosity")

	pingIntv = flag.Int("i", 60, "ping interval (second)")
	testMode = flag.String("mode", "http", "test mode: http, ping")
	pingTimeout = flag.Int("t", 3000, "ping timeout (Millisecond)")

	pingUrl = flag.String("host", "8.8.4.4;168.95.1.1;google.com;facebook.com", "url to ping (spare by ';')")
	httpUrl = flag.String("http", "http://google.com;http://facebook.com", "http url to try (spare by ';')")

	powerDownTimeout = flag.Int("pdr", -1, "no network wait for auto poweroff (second, default: 30 minute, 1800 sec )")
)

func main() {

	log.SetFlags(log.Ldate|log.Ltime)
	flag.Parse()
//	runtime.GOMAXPROCS(runtime.NumCPU())

	pinglist := strings.Split(*pingUrl, ";")
	Vln(1, "[pingUrl]", len(pinglist), pinglist)

	httplist := strings.Split(*httpUrl, ";")
	Vln(1, "[httpUrl]", len(httplist), httplist)

/*	cmdPing("127.0.0.1")
	webPing("http://google.com", 3 * time.Second)
	webPing("http://www.google.com", 3 * time.Second)
	webPing("http://facebook.com", 3 * time.Second)
	webPing("http://134.208.0.206/index", 3 * time.Second)
*/
	var pwdTmr *time.Timer
	for {
		var ok bool

		ok = tryPing(pinglist, cmdPing)
		if ok {
			goto WAIT
		}

		ok = tryPing(httplist, webPing)
		if ok {
			goto WAIT
		}


WAIT:
		if !ok {
			if *powerDownTimeout >= 0 {
				// set poweroff timer
				pwdTmr = time.AfterFunc(time.Duration(*powerDownTimeout) * time.Second, poweroff)
			}
		} else {
			if pwdTmr != nil {
				pwdTmr.Stop()
			}
		}
		time.Sleep(time.Duration(*pingIntv) * time.Second)
	}
}

func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}

func tryPing(list []string, fn (func(string, time.Duration) (bool)) ) (bool) {
	timeout := time.Duration(*pingTimeout) * time.Millisecond

	for _, addr := range list {
		ret := fn(addr, timeout)
		if ret {
			return true
		}
	}

	return false
}

func cmdPing(testurl string, timeout time.Duration) (bool) {
	cmd := exec.Command("ping", "-c", "1", testurl)
	out, err := cmd.CombinedOutput()
	if err != nil {
		Vln(2, "[cmdPing]err", testurl, err)
	}
	Vln(3, "[ping]", string(out))

	return cmd.ProcessState.Success()
}

func webPing(testurl string, timeout time.Duration) (ok bool) {
	ErrRedirect := errors.New("stopped after 1 redirects")
	client := http.Client {
		CheckRedirect: func (req *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				return ErrRedirect
			}
			return nil
		},
		Timeout: timeout,
	}
	out, err := client.Get(testurl)
	if err == nil {
		ok = true
	} else if err.(*url.Error).Err == ErrRedirect {
		ok = true
	} else {
		Vln(2, "[webPing]err", testurl, err)
//		log.Printf("[webPing] %#v %#v %#v", err, err.(*url.Error).Err, ErrRedirect)
	}
	Vln(3, "[http]", ok, out)

	return ok
}

func poweroff() {
	cmd := exec.Command("sudo", "poweroff")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	Vln(1, "[poweroff]", string(out))
}


