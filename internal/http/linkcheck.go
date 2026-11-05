package http

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tomek7667/links/internal/domain"
)

const (
	linkCheckTimeout     = 4 * time.Second
	linkCheckCacheTTL    = 15 * time.Second
	linkCheckConcurrency = 8
)

type LinkStatus struct {
	Up      bool   `json:"up"`
	Code    int    `json:"code,omitempty"`
	Ms      int64  `json:"ms"`
	Error   string `json:"error,omitempty"`
	Checked int64  `json:"checked"`
}

type cachedStatus struct {
	status LinkStatus
	at     time.Time
}

// linkChecker reports whether link targets respond. It runs on the server so
// it is not blocked by CORS and sees the network the way the server does.
type linkChecker struct {
	client *http.Client
	dialer *net.Dialer

	mu    sync.Mutex
	cache map[string]cachedStatus
}

func newLinkChecker() *linkChecker {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: dialer.DialContext,
		// Self-hosted services often use self-signed certificates; this only
		// checks reachability and never reads or trusts the response body.
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: linkCheckTimeout,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       30 * time.Second,
	}
	return &linkChecker{
		dialer: dialer,
		client: &http.Client{
			Transport: transport,
			Timeout:   linkCheckTimeout + time.Second,
			// A redirect (e.g. to a login page) already proves the service is up.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		cache: make(map[string]cachedStatus),
	}
}

// CheckAll returns statuses keyed by link ID. Links that can't be checked
// (e.g. mailto:) are left out.
func (c *linkChecker) CheckAll(ctx context.Context, links []domain.Link, ip, host string) map[string]LinkStatus {
	out := make(map[string]LinkStatus, len(links))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, linkCheckConcurrency)

	for _, l := range links {
		target := domain.ResolvePlaceholders(l.Url, ip, host)
		wg.Add(1)
		go func(id, target string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if st, ok := c.check(ctx, target); ok {
				mu.Lock()
				out[id] = st
				mu.Unlock()
			}
		}(l.ID, target)
	}
	wg.Wait()
	c.pruneCache()
	return out
}

func (c *linkChecker) check(ctx context.Context, target string) (LinkStatus, bool) {
	c.mu.Lock()
	if cached, ok := c.cache[target]; ok && time.Since(cached.at) < linkCheckCacheTTL {
		c.mu.Unlock()
		return cached.status, true
	}
	c.mu.Unlock()

	var st LinkStatus
	lower := strings.ToLower(target)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		st = c.checkHTTP(ctx, target)
	default:
		// Other schemes (ssh://, smb://, ...): a TCP connect is the best we can do.
		p := domain.SplitURL(target)
		if p.Scheme == "" || p.Port == "" {
			return LinkStatus{}, false
		}
		st = c.checkTCP(ctx, net.JoinHostPort(strings.Trim(p.Host, "[]"), p.Port))
	}
	st.Checked = time.Now().UnixMilli()

	c.mu.Lock()
	c.cache[target] = cachedStatus{status: st, at: time.Now()}
	c.mu.Unlock()
	return st, true
}

func (c *linkChecker) checkHTTP(ctx context.Context, target string) LinkStatus {
	ctx, cancel := context.WithTimeout(ctx, linkCheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return LinkStatus{Error: "invalid url"}
	}
	req.Header.Set("User-Agent", "linksserver-status/1")

	start := time.Now()
	resp, err := c.client.Do(req)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return LinkStatus{Ms: ms, Error: shortNetError(err)}
	}
	_ = resp.Body.Close()
	// Any HTTP answer (even 401/404) means something is listening.
	return LinkStatus{Up: true, Code: resp.StatusCode, Ms: ms}
}

func (c *linkChecker) checkTCP(ctx context.Context, addr string) LinkStatus {
	ctx, cancel := context.WithTimeout(ctx, linkCheckTimeout)
	defer cancel()
	start := time.Now()
	conn, err := c.dialer.DialContext(ctx, "tcp", addr)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return LinkStatus{Ms: ms, Error: shortNetError(err)}
	}
	_ = conn.Close()
	return LinkStatus{Up: true, Ms: ms}
}

func (c *linkChecker) pruneCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.cache {
		if time.Since(v.at) > 10*linkCheckCacheTTL {
			delete(c.cache, k)
		}
	}
}

func shortNetError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "connection refused"
	case strings.Contains(msg, "no such host"):
		return "unknown host"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "timed out"
	case strings.Contains(msg, "no route to host"), strings.Contains(msg, "network is unreachable"):
		return "unreachable"
	}
	if len(msg) > 80 {
		msg = msg[:80] + "…"
	}
	return msg
}
