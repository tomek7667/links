package domain

import "strings"

// URLParts is a lenient split of a URL into its pieces. Unlike net/url it
// accepts placeholders such as "{ip}" in the host, which is what lets the
// dashboard edit or replace the host/IP part of a link on its own.
//
// The same splitting rules are implemented in ui/assets/app.js (splitUrl).
type URLParts struct {
	Scheme   string // "http://" (including "://"), or empty
	UserInfo string // "user:pass@" (including "@"), or empty
	Host     string // "192.168.1.20", "{ip}", "[::1]", ...
	Port     string // "8096" (without ":"), or empty
	Rest     string // path, query and fragment, starting with "/", "?" or "#"
}

func HasScheme(raw string) bool {
	i := strings.Index(raw, "://")
	return i > 0 && isScheme(raw[:i])
}

func isScheme(s string) bool {
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return s != ""
}

func SplitURL(raw string) URLParts {
	var p URLParts
	s := raw
	if HasScheme(s) {
		i := strings.Index(s, "://")
		p.Scheme, s = s[:i+3], s[i+3:]
	}

	end := strings.IndexAny(s, "/?#")
	if end < 0 {
		end = len(s)
	}
	authority := s[:end]
	p.Rest = s[end:]

	if at := strings.LastIndex(authority, "@"); at >= 0 {
		p.UserInfo, authority = authority[:at+1], authority[at+1:]
	}

	if strings.HasPrefix(authority, "[") {
		if e := strings.Index(authority, "]"); e >= 0 {
			p.Host = authority[:e+1]
			if after := authority[e+1:]; strings.HasPrefix(after, ":") {
				p.Port = after[1:]
			}
			return p
		}
	}
	if c := strings.LastIndex(authority, ":"); c >= 0 {
		p.Host, p.Port = authority[:c], authority[c+1:]
	} else {
		p.Host = authority
	}
	return p
}

func (p URLParts) String() string {
	var b strings.Builder
	b.WriteString(p.Scheme)
	b.WriteString(p.UserInfo)
	b.WriteString(p.Host)
	if p.Port != "" {
		b.WriteByte(':')
		b.WriteString(p.Port)
	}
	b.WriteString(p.Rest)
	return b.String()
}

// ResolvePlaceholders substitutes {ip} and {host} in a link URL.
func ResolvePlaceholders(raw, ip, host string) string {
	if !strings.Contains(raw, "{") {
		return raw
	}
	return strings.NewReplacer(PlaceholderIP, ip, PlaceholderHost, host).Replace(raw)
}
