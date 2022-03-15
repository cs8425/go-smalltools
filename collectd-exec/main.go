/*

https://openwrt.org/docs/guide-user/perf_and_log/statistic.custom

openwrt:
	data type: /usr/share/collectd/types.db


*/
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"strings"
	"time"
)

type TestUrl struct {
	Url          string `json:"url,omitempty"`
	Name         string `json:"name,omitempty"`
	SkipRedirect bool   `json:"skip_redirect,omitempty"`
}

var (
	COLLECTD_INTERVAL = 15 * 1000 * time.Millisecond
	COLLECTD_HOSTNAME = os.Getenv("COLLECTD_HOSTNAME")

	urls = []*TestUrl{
		{"http://google.com", "google", true},
		{"http://facebook.com", "facebook", true},
		{"http://cloudflare.com", "cloudflare", true},

		// {"http://hnd-jp-ping.vultr.com", "vultr-jp", true},
		// {"http://hnd-jp-ping.vultr.com/assets/css/custom.min.css", "vultr-jp", true},
		// {"http://sgp-ping.vultr.com/assets/css/custom.min.css", "vultr-sg", true},
		// {"http://lax-ca-us-ping.vultr.com/assets/css/custom.min.css", "vultr-la", true},
		// {"http://tx-us-ping.vultr.com/assets/css/custom.min.css", "vultr-tx", true},
	}
)

// env $COLLECTD_INTERVAL second
// PUTVAL "<yourhostname>/exec-<instance>/<datatype>[-name]" [interval=X] <time>:<Y>
// PUTVAL "<yourhostname>/<lua>-<instance>/<datatype>-<name>" [interval=X] <time>:<Y>
// PUTVAL "phobos/exec-environmental/temperature-cpu" interval=30 N:88.4
// PUTVAL "phobos/exec-cpufreq/frequency-cpu0" interval=30 N:2300
func main() {

	fmt.Println("#[PATH]", os.Getenv("PATH"))
	fmt.Println("#[COLLECTD_HOSTNAME]", os.Getenv("COLLECTD_HOSTNAME"))
	fmt.Println("#[COLLECTD_INTERVAL]", os.Getenv("COLLECTD_INTERVAL"))

	if COLLECTD_HOSTNAME == "" {
		COLLECTD_HOSTNAME = "local"
	}
	intv, err := strconv.ParseFloat(os.Getenv("COLLECTD_INTERVAL"), 32)
	if err == nil {
		COLLECTD_INTERVAL = time.Duration(intv) * time.Second
	}

	if len(os.Args) >= 2 {
		var err error
		urls, err = LoadUrls(os.Args[1])
		if err != nil {
			fmt.Println("#[urls][err]", err)
		}
	}

	tr := NewTraceHttp()
	for {
		printCPUFreq()

		for _, info := range urls {
			err := tr.Get(info.Url, info.SkipRedirect)
			if err != nil {
				fmt.Println("#[err]", err)
			}
			tr.PutVal(COLLECTD_HOSTNAME, COLLECTD_INTERVAL, info.Name)
		}

		time.Sleep(COLLECTD_INTERVAL)
	}
}

func LoadUrls(fp string) ([]*TestUrl, error) {
	fd, err := os.Open(fp) // For read access.
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	urls := make([]*TestUrl, 0, 8)
	err = json.NewDecoder(fd).Decode(&urls)
	if err != nil {
		return nil, err
	}
	return urls, nil
}

func getInt(p string) (int64, error) {
	text, err := ioutil.ReadFile(p)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseInt(strings.TrimSpace(string(text)), 10, 64)
	return num, err
}

func getCPUFreq(num int) int64 {
	//"/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"
	basePath := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", num)
	freq, err := getInt(basePath)
	if err != nil {
		return -1
	}
	return freq
}

func printCPUFreq() {
	//"/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"
	const basePath = "/sys/devices/system/cpu/"
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return
	}

	for _, f := range files {
		name := f.Name()
		freq, err := getInt(basePath + name + "/cpufreq/scaling_cur_freq")
		if err != nil {
			continue
		}
		// fmt.Println("[freq]", name, Vfreq(freq))
		fmt.Printf("PUTVAL \"%v/exec-cpufreq/frequency-%v\" interval=%v N:%v\n", COLLECTD_HOSTNAME, name, COLLECTD_INTERVAL, freq/1000)
	}
}

func Vfreq(freq int64) (ret string) {
	var tmp float64 = float64(freq)
	var s string = " "

	switch {
	case freq < int64(1000):
		s = "k"

	case freq < int64(1000000):
		tmp = tmp / float64(1000)
		s = "M"

	case freq < int64(1000000000):
		tmp = tmp / float64(1000000)
		s = "G"

	}
	ret = fmt.Sprintf("%03.2f %sHz", tmp, s)
	return
}

func VTemp(temp int64) (ret string) {
	var tmp float64 = float64(temp) / 1000.0
	ret = fmt.Sprintf("%03.2f C", tmp)
	return
}

type TraceHttp struct {
	httptrace.ClientTrace

	start        time.Time
	connect      time.Time
	dns          time.Time
	tlsHandshake time.Time

	dnsDt   time.Duration
	connDt  time.Duration
	tlsDt   time.Duration
	ttfb    time.Duration
	totalDt time.Duration
}

func (th *TraceHttp) zero() {
	th.dnsDt = 0
	th.connDt = -1 * time.Millisecond
	th.tlsDt = 0
	th.ttfb = -1 * time.Millisecond
	th.totalDt = 0
}

func (th *TraceHttp) GetConn(hostPort string)             { th.start = time.Now() }
func (th *TraceHttp) DNSStart(dsi httptrace.DNSStartInfo) { th.dns = time.Now() }
func (th *TraceHttp) DNSDone(ddi httptrace.DNSDoneInfo) {
	if ddi.Err == nil {
		th.dnsDt = time.Since(th.dns)
	} else {
		th.dnsDt = -1 * time.Millisecond
	}
}
func (th *TraceHttp) ConnectStart(network, addr string) { th.connect = time.Now() }
func (th *TraceHttp) ConnectDone(network, addr string, err error) {
	if err == nil {
		th.connDt = time.Since(th.connect)
	}
}
func (th *TraceHttp) TLSHandshakeStart() { th.tlsHandshake = time.Now() }
func (th *TraceHttp) TLSHandshakeDone(cs tls.ConnectionState, err error) {
	if err == nil {
		th.tlsDt = time.Since(th.tlsHandshake)
	} else {
		th.tlsDt = -1 * time.Millisecond
		// fmt.Println("[err]TLSHandshakeDone", err)
	}
}
func (th *TraceHttp) GotFirstResponseByte() { th.ttfb = time.Since(th.start) } // TTFB

func (th *TraceHttp) PutVal(hostname string, interval time.Duration, name string) {
	nameStr := "request"
	if name != "" {
		nameStr = "request-" + name
	}
	// latency ping
	fmt.Printf("PUTVAL \"%v/exec-%v/latency-%v\" interval=%v N:%v\n", hostname, nameStr, "Connect", interval, printMS(th.connDt))
	fmt.Printf("PUTVAL \"%v/exec-%v/latency-%v\" interval=%v N:%v\n", hostname, nameStr, "TTFB", interval, printMS(th.ttfb))
	fmt.Printf("PUTVAL \"%v/exec-%v/latency-%v\" interval=%v N:%v\n", hostname, nameStr, "DNS", interval, printMS(th.dnsDt))
	fmt.Printf("PUTVAL \"%v/exec-%v/latency-%v\" interval=%v N:%v\n", hostname, nameStr, "total", interval, printMS(th.totalDt))
}

func NewTraceHttp() *TraceHttp {
	trace := &TraceHttp{
		// start: time.Now(),
	}
	trace.ClientTrace = httptrace.ClientTrace{
		GetConn: trace.GetConn,

		DNSStart: trace.DNSStart,
		DNSDone:  trace.DNSDone,

		ConnectStart: trace.ConnectStart,
		ConnectDone:  trace.ConnectDone,

		TLSHandshakeStart: trace.TLSHandshakeStart,
		TLSHandshakeDone:  trace.TLSHandshakeDone,

		GotFirstResponseByte: trace.GotFirstResponseByte,
	}
	return trace
}

func (th *TraceHttp) Get(url string, skipRedirect bool) error {
	th.zero()

	req, _ := http.NewRequest("GET", url, nil)

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 20 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 2500 * time.Millisecond,
		},
	}
	if skipRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	}
	defer client.CloseIdleConnections()

	// transport := &http.Transport{
	// 	Proxy: http.ProxyFromEnvironment,
	// 	DialContext: (&net.Dialer{
	// 		Timeout:   15 * time.Second,
	// 		KeepAlive: 20 * time.Second,
	// 	}).DialContext,
	// 	TLSHandshakeTimeout: 10 * time.Second,
	// }
	// defer transport.CloseIdleConnections()

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &th.ClientTrace))
	// res, err := transport.RoundTrip(req)
	res, err := client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		th.totalDt = -1 * time.Millisecond
		return err
	}

	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		th.totalDt = -1 * time.Millisecond
		return err
	}

	th.totalDt = time.Since(th.start)
	return err
}

func printMS(dt time.Duration) string {
	if dt < 0 {
		return "U"
	}
	return fmt.Sprintf("%v", float32(dt.Microseconds())/1000.0)
}
