package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravwell/gravwell/v3/ingest"
	"github.com/gravwell/gravwell/v3/ingest/entry"
	"github.com/tatsushid/go-fastping"
)

func startPinger(ctx context.Context, igst *ingest.IngestMuxer, tag entry.EntryTag, interval, timeout time.Duration, hosts []string) (*fastping.Pinger, error) {

	p := fastping.NewPinger()
	p.MaxRTT = timeout
	rt := newResptracker()
	for _, v := range hosts {
		ra, err := net.ResolveIPAddr(`ip4:icmp`, v)
		if err != nil {
			return nil, err
		}
		p.AddIPAddr(ra)
		rt.AddIP(v, ra)
	}
	p.OnRecv = rt.Update
	//p.OnIdle = rt.Done
	go run(ctx, p, rt, interval, igst, tag)
	return p, nil
}

func run(ctx context.Context, p *fastping.Pinger, rt *resptracker, interval time.Duration, igst *ingest.IngestMuxer, tag entry.EntryTag) {
	var lastError, err error
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		now := time.Now()
		rt.Reset()
		if err = p.Run(); err == nil {
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
					Tag:  tag,
					Data: []byte(msg),
				}
				return igst.WriteEntry(ent)
			})
			//always reset last error on success
			lastError = nil
		} else {
			//check last error so we only throw the message once if there is an error
			//this is almost always due to permissions errors
			if lastError == nil {
				msg := fmt.Sprintf("%v\tERROR\t%v", now.UTC().Format(time.RFC3339), err)
				ent := &entry.Entry{
					TS:   entry.FromStandard(now),
					Tag:  tag,
					Data: []byte(msg),
				}
				igst.WriteEntry(ent)
				lastError = err
			}
		}
		sleepContext(ctx, interval-time.Since(now))
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
	rt.mp[ip.String()] = resp{h: host}
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

type cbfunc func(ts time.Time, host, ip string, d time.Duration) error

func (rt *resptracker) Status(cb cbfunc) {
	rt.Lock()
	for k, v := range rt.mp {
		if err := cb(v.ts.UTC(), v.h, k, v.d); err != nil {
			break
		}
	}
	rt.Unlock()
}
