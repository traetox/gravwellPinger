package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravwell/gravwell/v3/ingest"
	"github.com/gravwell/gravwell/v3/ingest/entry"
	"github.com/gravwell/gravwell/v3/ingesters/args"
	"github.com/gravwell/gravwell/v3/ingesters/version"
	"github.com/tatsushid/go-fastping"
)

const (
	ingesterName = `pinger`
)

var (
	rttF   = flag.String("ping-timeout", "2000ms", "ICMP RTT imeout")
	ver    = flag.Bool("version", false, "Print version and exit")
	srcOvr = flag.String("source-override", "", "Override source with address, hash, or integeter")

	igstF = flag.String
	to    time.Duration
)

func init() {
	var err error
	flag.Parse()
	if *ver {
		version.PrintVersion(os.Stdout)
		ingest.PrintVersion(os.Stdout)
		os.Exit(0)
	}
	if to, err = time.ParseDuration(*rttF); err != nil {
		log.Fatalf("Invalid timeout %s: %v\n", *rttF, err)
	}
}

func main() {
	a, err := args.Parse()
	if err != nil {
		log.Fatalf("Invalid arguments: %v\n", err)
	}
	if len(a.Tags) != 1 {
		log.Fatal("File oneshot only accepts a single tag")
	}
	hosts := flag.Args()
	if len(hosts) == 0 {
		log.Fatalf("At least one endpoint is required\n")
	}

	igCfg := ingest.UniformMuxerConfig{
		Destinations:    a.Conns,
		Tags:            a.Tags,
		Auth:            a.IngestSecret,
		IngesterName:    ingesterName,
		IngesterVersion: version.GetVersion(),
		IngesterUUID:    uuid.New().String(),
		IngesterLabel:   `Gravwell ICMP Checker`,
	}
	igst, err := ingest.NewUniformMuxer(igCfg)
	if err != nil {
		log.Fatal("failed build our ingest system", err)
		return
	}

	//fire up a uniform muxer
	if err := igst.Start(); err != nil {
		log.Fatalf("Failed to start ingest muxer: %v\n", err)
	}
	if err := igst.WaitForHot(a.Timeout); err != nil {
		log.Fatalf("Failed to wait for hot connection: %v\n", err)
	}
	tag, err := igst.GetTag(a.Tags[0])
	if err != nil {
		log.Fatalf("Failed to resolve tag %s: %v\n", a.Tags[0], err)
	}

	p := fastping.NewPinger()
	rt := newResptracker()
	for _, v := range hosts {
		ra, err := net.ResolveIPAddr(`ip4:icmp`, v)
		if err != nil {
			log.Fatalf("Failed to resolve %v: %v\n", v, err)
		}
		p.AddIPAddr(ra)
		rt.AddIP(v, ra)
	}
	p.OnRecv = rt.Update
	//p.OnIdle = rt.Done

	ec := make(chan error, 1)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	go run(p, rt, to, igst, tag, ec)
	select {
	case err := <-ec:
		log.Printf("Ping failed: %v\n", err)
	case <-c:
		log.Println("Exiting")
		p.Stop()
	}
	if err = igst.Sync(a.Timeout); err != nil {
		log.Fatalf("Failed to sync ingest muxer: %v\n", err)
	}
	if err := igst.Close(); err != nil {
		log.Fatalf("Failed to close the ingest muxer: %v\n", err)
	}
}

func run(p *fastping.Pinger, rt *resptracker, interval time.Duration, igst *ingest.IngestMuxer, tag entry.EntryTag, ec chan error) {
	defer close(ec)
	src, err := igst.SourceIP()
	if err != nil {
		ec <- err
		return
	}
	for {
		now := time.Now()
		rt.Reset()
		if err := p.Run(); err != nil {
			ec <- err
			return
		}
		rt.Status(func(ts time.Time, host, ip string, dur time.Duration) error {
			tss := ts.UTC().Format(time.RFC3339)
			var msg string
			if dur <= 0 {
				msg = fmt.Sprintf("%v\tICMP\t%v\t%v\tTIMEOUT", tss, host, ip)
			} else {
				ms := float64(dur.Microseconds()) / 1000.0
				msg = fmt.Sprintf("%v\tICMP\t%v\t%v\t%f", tss, host, ip, ms)
			}
			ent := &entry.Entry{
				TS:   entry.FromStandard(ts),
				SRC:  src,
				Tag:  tag,
				Data: []byte(msg),
			}
			return igst.WriteEntry(ent)
		})
		sleepDur := interval - time.Since(now)
		if sleepDur > 0 {
			time.Sleep(sleepDur)
		}
	}
}

type resp struct {
	ts time.Time
	d  time.Duration
	h  string
}

type resptracker struct {
	sync.Mutex
	mp map[string]resp
}

func newResptracker() *resptracker {
	return &resptracker{
		mp: make(map[string]resp),
	}
}

func (rt *resptracker) AddIP(host string, ip *net.IPAddr) error {
	rt.Lock()
	defer rt.Unlock()
	if _, ok := rt.mp[ip.String()]; ok {
		return fmt.Errorf("%v already exists", ip)
	}
	rt.mp[ip.String()] = resp{d: 0, h: host}
	return nil
}

func (rt *resptracker) Reset() {
	rt.Lock()
	for k, v := range rt.mp {
		v.d = 0
		v.ts = time.Now()
		rt.mp[k] = v
	}
	rt.Unlock()
}

func (rt *resptracker) Update(ip *net.IPAddr, d time.Duration) {
	rt.Lock()
	if v, ok := rt.mp[ip.String()]; ok {
		v.d = d
		rt.mp[ip.String()] = v
	}
	rt.Unlock()
}

type cbfunc func(ts time.Time, host, ip string, dur time.Duration) error

func (rt *resptracker) Status(cb cbfunc) {
	rt.Lock()
	for k, v := range rt.mp {
		if err := cb(v.ts.UTC(), v.h, k, v.d); err != nil {
			break
		}
	}
	rt.Unlock()
}
