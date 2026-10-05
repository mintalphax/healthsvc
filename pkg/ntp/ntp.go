// Package ntp implements a minimal SNTPv4 client used to obtain a trusted
// time even when the local clock has been tampered with.
package ntp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
)

const (
	ntpEpochOffset = 2208988800 // seconds between 1900-01-01 and 1970-01-01
	packetSize     = 48

	clientMode  = 3
	serverMode  = 4
	broadcastFl = 5
)

// Client queries a list of NTP servers with retries.
type Client struct {
	Servers       []string
	MaxRetries    int
	RetryInterval time.Duration
	Timeout       time.Duration
	// OverallBudget bounds the total time one GetTime call may spend across
	// all servers and retries, so a flaky network cannot stall the scheduler
	// loop for minutes. Zero means 15 seconds.
	OverallBudget time.Duration
}

// Result is one successful time query.
type Result struct {
	Time   time.Time     // current trusted time
	Offset time.Duration // trusted - local clock
	RTT    time.Duration
	Server string
}

var errNoServers = errors.New("no ntp servers configured")

// GetTime tries every configured server, up to MaxRetries rounds, and returns
// the first successful response. It never spends more than OverallBudget
// (default 15s) in total.
func (c *Client) GetTime(now time.Time) (*Result, error) {
	if len(c.Servers) == 0 {
		return nil, errNoServers
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	retries := c.MaxRetries
	if retries <= 0 {
		retries = 1
	}
	budget := c.OverallBudget
	if budget <= 0 {
		budget = 15 * time.Second
	}
	deadline := time.Now().Add(budget)

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		for _, server := range c.Servers {
			if time.Now().After(deadline) {
				if lastErr == nil {
					lastErr = errors.New("ntp time budget exhausted")
				}
				return nil, fmt.Errorf("%w (budget %v)", lastErr, budget)
			}
			res, err := c.query(server, timeout)
			if err == nil {
				return res, nil
			}
			lastErr = fmt.Errorf("ntp %s: %w", server, err)
		}
		if attempt < retries-1 && c.RetryInterval > 0 {
			time.Sleep(c.RetryInterval)
		}
	}
	return nil, lastErr
}

func (c *Client) query(server string, timeout time.Duration) (*Result, error) {
	addr := server
	if !strings.Contains(addr, ":") { // no port given
		addr = addr + ":123"
	}
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := make([]byte, packetSize)
	req[0] = 0<<6 | 4<<3 | clientMode                                // LI=0, VN=4, Mode=client
	binary.BigEndian.PutUint32(req[24:28], toNTPSeconds(time.Now())) // originate ts

	t1 := time.Now()
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	resp := make([]byte, packetSize)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	t4 := time.Now()
	if n < packetSize {
		return nil, fmt.Errorf("short response %d bytes", n)
	}

	li := resp[0] >> 6
	mode := resp[0] & 0x07
	stratum := resp[1]
	if mode != serverMode && mode != broadcastFl {
		return nil, fmt.Errorf("unexpected mode %d", mode)
	}
	if li == 3 {
		return nil, errors.New("server clock not synchronized")
	}
	if stratum == 0 {
		return nil, errors.New("kiss-of-death response (stratum 0)")
	}

	t2 := fromNTPTime(resp[32:40]) // server receive
	t3 := fromNTPTime(resp[40:48]) // server transmit
	if t3.IsZero() || t2.IsZero() || t3.Year() < 1990 || t3.Year() > 2100 {
		return nil, errors.New("invalid transmit time in response")
	}

	offset := ((t2.Sub(t1) + t3.Sub(t4)) / 2).Round(time.Millisecond)
	rtt := (t4.Sub(t1) - t3.Sub(t2)).Round(time.Millisecond)
	if offset > math.MaxInt64/2 || offset < math.MinInt64/2 {
		return nil, errors.New("implausible time offset")
	}
	return &Result{
		Time:   t4.Add(offset),
		Offset: offset,
		RTT:    rtt,
		Server: server,
	}, nil
}

func toNTPSeconds(t time.Time) uint32 {
	return uint32(t.Unix() + ntpEpochOffset)
}

func fromNTPTime(b []byte) time.Time {
	sec := binary.BigEndian.Uint32(b[0:4])
	frac := binary.BigEndian.Uint32(b[4:8])
	if sec == 0 && frac == 0 {
		return time.Time{}
	}
	unix := int64(sec) - ntpEpochOffset
	nsec := int64(frac) * 1e9 >> 32
	return time.Unix(unix, nsec).UTC()
}
