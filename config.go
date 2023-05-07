package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/google/uuid"
	"github.com/gravwell/gravwell/v3/ingest"
	"github.com/gravwell/gravwell/v3/ingest/config"
)

const (
	defaultTimeout  = time.Second
	defaultInterval = 10 * time.Second
	defaultTag      = `pinger`
)

type cfgType struct {
	Global config.IngestConfig
	HTTP   httpCfg
	ICMP   baseCfg
}

type httpCfg struct {
	baseCfg
	Follow_Redirects bool
	Allow_Bad_TLS    bool
}

type baseCfg struct {
	Timeout  string
	Interval string
	Target   []string
	Tag_Name string
}

type icmpCfg struct {
	baseCfg
}

func GetConfig(path, overlayPath string) (*cfgType, error) {
	//read into the intermediary type to maintain backwards compatibility with the old system
	var cr cfgType
	if err := config.LoadConfigFile(&cr, path); err != nil {
		return nil, err
	} else if err = config.LoadConfigOverlays(&cr, overlayPath); err != nil {
		return nil, err
	}

	if err := cr.Global.Verify(); err != nil {
		return nil, err
	} else if err = cr.HTTP.validate(); err != nil {
		return nil, err
	} else if err = cr.ICMP.validate(); err != nil {
		return nil, err
	}

	// Verify and set UUID
	if _, ok := cr.Global.IngesterUUID(); !ok {
		id := uuid.New()
		if err := cr.Global.SetIngesterUUID(id, path); err != nil {
			return nil, err
		}
		if id2, ok := cr.Global.IngesterUUID(); !ok || id != id2 {
			return nil, errors.New("Failed to set a new ingester UUID")
		}
	}
	return &cr, nil
}

func (c icmpCfg) validate() (err error) {
	if err = c.baseCfg.validate(); err != nil {
		return fmt.Errorf("ICMP %w", err)
	}

	//validate all the HTTP targets are valid URLs
	for _, v := range c.Target {
		//try to parse as an IP
		if net.ParseIP(v) == nil {
			//try as a hostname
			if !govalidator.IsDNSName(v) {
				return fmt.Errorf("ICMP Invalid Hostname %q - size", v)
			}
		}
	}
	return nil
}

func (c httpCfg) validate() (err error) {
	if err = c.baseCfg.validate(); err != nil {
		return fmt.Errorf("HTTP %w", err)
	}

	//validate all the HTTP targets are valid URLs
	for _, v := range c.Target {
		var uri *url.URL
		if uri, err = url.Parse(v); err != nil {
			return fmt.Errorf("Invalid HTTP URL %q - %w", v, err)
		} else if uri == nil {
			return fmt.Errorf("Invalid HTTP URL %q", v)
		} else if uri.Host == `` {
			return fmt.Errorf("Invalid HTTP URL %q - missing host", v)
		}
	}

	return nil
}

func (c baseCfg) validate() (err error) {
	//first validate timeouts
	if c.Timeout != `` {
		if _, err = time.ParseDuration(c.Timeout); err != nil {
			return fmt.Errorf("Invalid Timeout %q - %w", c.Timeout, err)
		}
	}
	if c.Interval != `` {
		if _, err = time.ParseDuration(c.Interval); err != nil {
			return fmt.Errorf("Invalid Interval %q - %w", c.Interval, err)
		}
	}

	if c.Tag_Name != `` {
		if err = ingest.CheckTag(c.Tag_Name); err != nil {
			return fmt.Errorf("Invalid Tag-Name %q %w", c.Tag_Name, err)
		}
	}
	return nil
}

func (cr baseCfg) tag() string {
	if cr.Tag_Name == `` {
		return defaultTag
	}
	return cr.Tag_Name
}

func (cr baseCfg) timeout() time.Duration {
	if cr.Timeout == `` {
		return defaultTimeout
	}
	// validate already checked this
	to, _ := time.ParseDuration(cr.Timeout)
	return to
}

func (cr baseCfg) interval() time.Duration {
	if cr.Timeout == `` {
		return defaultInterval
	}
	// validate already checked this
	to, _ := time.ParseDuration(cr.Interval)
	return to
}

func (c *cfgType) Tags() (tags []string, err error) {

	tg := c.HTTP.tag()
	tags = append(tags, tg)
	if ntg := c.ICMP.tag(); ntg != tg {
		tags = append(tags, tg)
	}
	return
}

func (c *cfgType) IngestBaseConfig() config.IngestConfig {
	return c.Global
}
