package domain

import (
	"errors"
	"testing"
)

func TestSplitURLRoundTrip(t *testing.T) {
	cases := []struct {
		in               string
		host, port, rest string
	}{
		{"http://192.168.1.20:8096/web/index.html#!/home", "192.168.1.20", "8096", "/web/index.html#!/home"},
		{"https://{ip}:9443", "{ip}", "9443", ""},
		{"http://user:pw@nas.local/share?x=1", "nas.local", "", "/share?x=1"},
		{"http://[fe80::1]:8080/", "[fe80::1]", "8080", "/"},
		{"192.168.1.1", "192.168.1.1", "", ""},
		{"ssh://pi@10.0.0.5:22", "10.0.0.5", "22", ""},
	}
	for _, c := range cases {
		p := SplitURL(c.in)
		if p.Host != c.host || p.Port != c.port || p.Rest != c.rest {
			t.Errorf("SplitURL(%q) = host %q port %q rest %q; want %q %q %q", c.in, p.Host, p.Port, p.Rest, c.host, c.port, c.rest)
		}
		if got := p.String(); got != c.in {
			t.Errorf("SplitURL(%q).String() = %q; want round trip", c.in, got)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	ok := map[string]string{
		"192.168.1.20:8096":     "http://192.168.1.20:8096",
		"{ip}:3000/d/x":         "http://{ip}:3000/d/x",
		"localhost:8080/?r=a":   "http://localhost:8080/?r=a",
		" https://example.com":  "https://example.com",
		"mailto:me@example.com": "mailto:me@example.com",
	}
	for in, want := range ok {
		got, err := NormalizeURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "javascript:alert(1)", " JavaScript:alert(1)", "data:text/html,x", "http://a b", "http://"} {
		if _, err := NormalizeURL(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("NormalizeURL(%q) err = %v; want ErrInvalid", in, err)
		}
	}
}

func TestResolvePlaceholders(t *testing.T) {
	got := ResolvePlaceholders("http://{ip}:80/{host}", "10.0.0.2", "pi.local")
	if got != "http://10.0.0.2:80/pi.local" {
		t.Fatalf("got %q", got)
	}
}
