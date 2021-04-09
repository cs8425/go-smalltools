package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"runtime"
	"time"

	"bufio"
	"os"
	"strconv"
	"strings"
)

var (
	T     = flag.Float64("t", 2, "update time(s)")
	C     = flag.Uint("c", 0, "count (0 == unlimit)")
	Inter = flag.String("i", "*", "interface")

	verbosity = flag.Int("v", 2, "verbosity")
)

func ReadLines(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}
	return ret, nil
}

func getInt(p string) (int64, error) {
	text, err := ioutil.ReadFile(p)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseInt(strings.TrimSpace(string(text)), 10, 64)
	return num, err
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	flag.Parse()

	// runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GOMAXPROCS(1)

	i := *C
	if i > 0 {
		i += 1
	}

	if *T < 0.01 {
		*T = 0.01
	}

	nettop := NewNetTop()
	cputop := NewCPUUsage()
	//	start := time.Now()
	for {
		i -= 1
		if i == 0 {
			break
		}

		cpuDelta := cputop.Update()
		printCPU(cpuDelta)

		printTemp()

		delta, dt := nettop.Update()
		printNettop(delta, dt)

		// elapsed := time.Since(start)
		time.Sleep(time.Duration(*T*1000) * time.Millisecond)
		fmt.Println("============")
		// start = time.Now()
	}
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
		fmt.Println("[freq]", name, Vfreq(freq))
	}
}

func printTemp() {
	//"/sys/class/thermal/thermal_zone0/temp"
	const basePath = "/sys/class/thermal/"
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return
	}

	for _, f := range files {
		name := f.Name()
		temp, err := getInt(basePath + name + "/temp")
		if err != nil {
			continue
		}
		fmt.Println("[temp]", name, VTemp(temp))
	}
}

func printNettop(delta *NetStat, dt time.Duration) {
	dtf := dt.Seconds()
	for _, iface := range delta.Dev {
		stat := delta.Stat[iface]
		fmt.Printf("[iface]\t%v\tRx:%v\tTx:%v\n", iface, Vsize(stat.Rx, dtf), Vsize(stat.Tx, dtf))
	}
}

func printCPU(delta map[int]*CPUTime) {
	count := len(delta)
	if ct, ok := delta[-1]; ok {
		rate := (ct.NonIdleC * 10000) / ct.Total
		fmt.Printf("[cpu]\t%02.2f%%\n", float32(rate)/100.0)
	}
	for i := 0; i < count; i++ {
		ct, ok := delta[i]
		if !ok {
			continue
		}
		rate := (ct.NonIdleC * 10000) / ct.Total
		freq := getCPUFreq(i)
		if freq >= 0 {
			fmt.Printf("[cpu%v]\t%02.2f%%\t%v\n", i, float32(rate)/100.0, Vfreq(freq))
		} else {
			fmt.Printf("[cpu%v]\t%02.2f%%\n", i, float32(rate)/100.0)
		}
	}
	// for num, ct := range delta {
	// 	rate := (ct.NonIdleC * 10000) / ct.Total
	// 	fmt.Printf("[cpu][%v]\t%02.2f%%\n", num, float32(rate)/100.0)
	// }
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

type CPUUsage struct {
	delta map[int]*CPUTime
	last  map[int]*CPUTime
	t0    time.Time
	dt    time.Duration
}
type CPUTime struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOwait  uint64
	Irq     uint64
	SoftIrq uint64
	Steal   uint64
	Guest   uint64
	GNice   uint64

	IdleC    uint64 // idle + iowait
	NonIdleC uint64 // user + nice + system + irq + softirq + steal
	Total    uint64 // IdleC + NonIdleC
}

func NewCPUUsage() *CPUUsage {
	cu := &CPUUsage{
		delta: make(map[int]*CPUTime),
		last:  make(map[int]*CPUTime),
		t0:    time.Now(),
		dt:    1500 * time.Millisecond,
	}
	return cu
}
func (cu *CPUUsage) Update() map[int]*CPUTime {
	stat1 := cu.getInfo()
	cu.dt = time.Since(cu.t0)
	for num, ct := range stat1 {
		ct0, ok := cu.last[num]
		if !ok {
			continue
		}

		cd, ok := cu.delta[num]
		if !ok {
			cd = &CPUTime{}
		}
		cd.User = ct.User - ct0.User
		cd.Nice = ct.Nice - ct0.Nice
		cd.System = ct.System - ct0.System
		cd.Idle = ct.Idle - ct0.Idle
		cd.IOwait = ct.IOwait - ct0.IOwait
		cd.Irq = ct.Irq - ct0.Irq
		cd.SoftIrq = ct.SoftIrq - ct0.SoftIrq
		cd.Steal = ct.Steal - ct0.Steal
		cd.Guest = ct.Guest - ct0.Guest
		cd.GNice = ct.GNice - ct0.GNice

		cd.IdleC = ct.IdleC - ct0.IdleC
		cd.NonIdleC = ct.NonIdleC - ct0.NonIdleC
		cd.Total = ct.Total - ct0.Total

		cu.delta[num] = cd
		// Vln(5, "[cd]", num, cd)
	}
	cu.last = stat1
	cu.t0 = time.Now()
	return cu.delta
}
func (cu *CPUUsage) getInfo() map[int]*CPUTime {
	lines, _ := ReadLines("/proc/stat")
	ret := make(map[int]*CPUTime)

	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		cpuNum := -1 // all == -1
		cpuNumStr := strings.TrimPrefix(fields[0], "cpu")
		if cpuNumStr != "" {
			if num, err := strconv.ParseInt(cpuNumStr, 10, 64); err == nil {
				cpuNum = int(num)
			}
		}
		// Vln(5, "[cpu]", cpuNum, fields)

		c := &CPUTime{}
		if n, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			c.User = uint64(n)
		}
		if n, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
			c.Nice = uint64(n)
		}
		if n, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
			c.System = uint64(n)
		}
		if n, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			c.Idle = uint64(n)
		}
		if n, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
			c.IOwait = uint64(n)
		}
		if n, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
			c.Irq = uint64(n)
		}
		if n, err := strconv.ParseInt(fields[7], 10, 64); err == nil {
			c.SoftIrq = uint64(n)
		}

		count := len(fields)
		if count >= 9 {
			if n, err := strconv.ParseInt(fields[8], 10, 64); err == nil {
				c.Steal = uint64(n)
			}
		}
		if count >= 10 {
			if n, err := strconv.ParseInt(fields[9], 10, 64); err == nil {
				c.Guest = uint64(n)
			}
		}
		if count >= 11 {
			if n, err := strconv.ParseInt(fields[10], 10, 64); err == nil {
				c.GNice = uint64(n)
			}
		}

		c.IdleC = c.Idle + c.IOwait
		c.NonIdleC = c.User + c.Nice + c.System + c.Irq + c.SoftIrq + c.Steal
		c.Total = c.IdleC + c.NonIdleC

		ret[cpuNum] = c
	}

	return ret
}

type NetTop struct {
	delta     *NetStat
	last      *NetStat
	t0        time.Time
	dt        time.Duration
	Interface string
}

func NewNetTop() *NetTop {
	nt := &NetTop{
		delta:     NewNetStat(),
		last:      NewNetStat(),
		t0:        time.Now(),
		dt:        1500 * time.Millisecond,
		Interface: "*",
	}
	return nt
}

func (nt *NetTop) Update() (*NetStat, time.Duration) {
	stat1 := nt.getInfo()
	nt.dt = time.Since(nt.t0)

	// Vln(5, nt.last)
	for _, value := range stat1.Dev {
		t0, ok := nt.last.Stat[value]
		// fmt.Println("k:", key, " v:", value, ok)
		if !ok {
			continue
		}

		dev, ok := nt.delta.Stat[value]
		if !ok {
			nt.delta.Stat[value] = new(DevStat)
			dev = nt.delta.Stat[value]
			nt.delta.Dev = append(nt.delta.Dev, value)
		}
		t1 := stat1.Stat[value]
		dev.Rx = t1.Rx - t0.Rx
		dev.Tx = t1.Tx - t0.Tx
	}
	nt.last = &stat1
	nt.t0 = time.Now()

	return nt.delta, nt.dt
}

func (nt *NetTop) getInfo() (ret NetStat) {

	lines, _ := ReadLines("/proc/net/dev")

	ret.Dev = make([]string, 0)
	ret.Stat = make(map[string]*DevStat)

	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.Fields(strings.TrimSpace(fields[1]))

		//Vln(5, key, value)

		if nt.Interface != "*" && nt.Interface != key {
			continue
		}

		c := new(DevStat)
		// c := DevStat{}
		c.Name = key
		r, err := strconv.ParseInt(value[0], 10, 64)
		if err != nil {
			Vln(4, key, "Rx", value[0], err)
			break
		}
		c.Rx = uint64(r)

		t, err := strconv.ParseInt(value[8], 10, 64)
		if err != nil {
			Vln(4, key, "Tx", value[8], err)
			break
		}
		c.Tx = uint64(t)

		ret.Dev = append(ret.Dev, key)
		ret.Stat[key] = c
	}

	return
}

type NetStat struct {
	Dev  []string
	Stat map[string]*DevStat
}

func NewNetStat() *NetStat {
	return &NetStat{
		Dev:  make([]string, 0),
		Stat: make(map[string]*DevStat),
	}
}

type DevStat struct {
	Name string
	Rx   uint64
	Tx   uint64
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

func Vsize(bytes uint64, delta float64) (ret string) {
	var tmp float64 = float64(bytes) / delta
	var s string = " "

	bytes = uint64(tmp)

	switch {
	case bytes < uint64(2<<9):

	case bytes < uint64(2<<19):
		tmp = tmp / float64(2<<9)
		s = "K"

	case bytes < uint64(2<<29):
		tmp = tmp / float64(2<<19)
		s = "M"

	case bytes < uint64(2<<39):
		tmp = tmp / float64(2<<29)
		s = "G"

	case bytes < uint64(2<<49):
		tmp = tmp / float64(2<<39)
		s = "T"

	}
	ret = fmt.Sprintf("%06.2f %sB/s", tmp, s)
	return
}

func Vf(level int, format string, v ...interface{}) {
	if level <= *verbosity {
		log.Printf(format, v...)
	}
}
func Vln(level int, v ...interface{}) {
	if level <= *verbosity {
		log.Println(v...)
	}
}
