package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/gravwell/gravwell/v3/ingest"
	"github.com/gravwell/gravwell/v3/ingest/entry"
)

const (
	defaultHttpTimeout  = 5 * time.Second
	defaultHttpInterval = time.Minute
)

func startHttp(ctx context.Context, igst *ingest.IngestMuxer, tag entry.EntryTag, interval, timeout time.Duration, urls []string) (err error) {
	if timeout <= 0 {
		timeout = defaultHttpTimeout
	}
	if interval <= 0 {
		interval = defaultHttpInterval
	}
	for _, v := range urls {
		var uri *url.URL
		if uri, err = url.Parse(v); err != nil {
			return fmt.Errorf("Invalid URL %q - %w", v, err)
		} else {
			go httpRequestRoutine(ctx, igst, tag, interval, timeout, uri)
		}
	}

	return
}

func httpRequestRoutine(pctx context.Context, igst *ingest.IngestMuxer, tag entry.EntryTag, interval, timeout time.Duration, uri *url.URL) {
	for {
		select {
		case <-pctx.Done():
			return
		default:
		}
		var msg string
		now := time.Now()
		ctx, cf := context.WithTimeout(pctx, timeout)
		if headers, body, code, err := doRequest(ctx, uri); err != nil {
			//report error
			msg = fmt.Sprintf("%v\tERROR\t%v\t%v", now.UTC().Format(time.RFC3339), uri.String(), err)
		} else {
			//throw metrics
			msg = fmt.Sprintf("%v\tHTTP\t%v\t%d\t%v\t%v",
				now.UTC().Format(time.RFC3339),
				uri.String(),
				code,
				stringDur(headers),
				stringDur(body),
			)
			igst.WriteEntry(&entry.Entry{
				TS:   entry.FromStandard(now),
				Tag:  tag,
				Data: []byte(msg),
			})
		}
		cf()
		sleepContext(pctx, interval-time.Since(now))
	}
}

func doRequest(ctx context.Context, uri *url.URL) (hdur, bdur time.Duration, code int, err error) {
	var resp *http.Response
	req := http.Request{
		Method: http.MethodGet,
		URL:    uri,
		Cancel: ctx.Done(),
	}
	ts := time.Now()
	if resp, err = http.DefaultClient.Do(&req); err != nil {
		return
	}
	hdur = time.Since(ts)
	code = resp.StatusCode
	if _, err = io.Copy(ioutil.Discard, resp.Body); err != nil {
		return
	}
	bdur = time.Since(ts)
	resp.Body.Close()
	return
}

func stringDur(d time.Duration) string {
	if d <= 0 {
		return `TIMEOUT`
	}
	return fmt.Sprintf("%.02f", float64(d.Microseconds())/1000.0)
}
