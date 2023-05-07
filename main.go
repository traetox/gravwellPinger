package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gravwell/gravwell/v3/ingest/log"
	"github.com/gravwell/gravwell/v3/ingesters/base"
	"github.com/gravwell/gravwell/v3/ingesters/utils"
)

const (
	ingesterName            = `gravwell pinger`
	appName                 = `pinger`
	defaultConfigLoc        = `/opt/pinger/pinger.conf`
	defaultConfigOverlayLoc = `/opt/pinger/pinger.conf.d`
)

func main() {
	var cfg *cfgType
	ibc := base.IngesterBaseConfig{
		IngesterName:                 ingesterName,
		AppName:                      appName,
		DefaultConfigLocation:        defaultConfigLoc,
		DefaultConfigOverlayLocation: defaultConfigOverlayLoc,
		GetConfigFunc:                GetConfig,
	}
	ib, err := base.Init(ibc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get configuration %v\n", err)
		return
	} else if err = ib.AssignConfig(&cfg); err != nil || cfg == nil {
		fmt.Fprintf(os.Stderr, "failed to assign configuration %v %v\n", err, cfg == nil)
		return
	}

	igst, err := ib.GetMuxer()
	if err != nil {
		ib.Logger.FatalCode(0, "failed to get ingest connection", log.KVErr(err))
		return
	}
	defer igst.Close()
	if err := igst.WaitForHot(time.Second); err != nil {
		ib.Logger.FatalCode(0, "Failed to wait for hot connection", log.KVErr(err))
	}
	icmpTag, err := igst.NegotiateTag(cfg.ICMP.tag())
	if err != nil {
		ib.Logger.FatalCode(0, "Failed to resolve ICMP tag", log.KV("tag", cfg.ICMP.tag()), log.KVErr(err))
	}

	httpTag, err := igst.NegotiateTag(cfg.HTTP.tag())
	if err != nil {
		ib.Logger.FatalCode(0, "Failed to resolve HTTP tag", log.KV("tag", cfg.HTTP.tag()), log.KVErr(err))
	}

	ctx, cf := context.WithCancel(context.Background())
	p, err := startPinger(ctx, igst, icmpTag, cfg.ICMP.interval(), cfg.ICMP.timeout(), cfg.ICMP.Target)
	if err != nil {
		ib.Logger.FatalCode(0, "Failed to start pinging routine", log.KVErr(err))
	} else if err = startHttp(ctx, igst, httpTag, cfg.HTTP.interval(), cfg.HTTP.timeout(), cfg.HTTP.Target, cfg.HTTP.Allow_Bad_TLS, cfg.HTTP.Follow_Redirects); err != nil {
		ib.Logger.FatalCode(0, "Failed to start http test routine", log.KVErr(err))
	}
	utils.WaitForQuit()
	p.Stop()
	cf()

	if err = igst.Sync(time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync ingest muxer: %v\n", err)
	}
	if err := igst.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close the ingest muxer: %v\n", err)
	}
}
func sleepContext(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}
}
