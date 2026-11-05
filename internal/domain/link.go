package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
)

// Placeholders that may be used inside a link URL. They are resolved when the
// link is displayed or checked, so links keep working when the server IP changes.
const (
	PlaceholderIP   = "{ip}"   // the server IP (settings override or auto-detected)
	PlaceholderHost = "{host}" // the hostname the dashboard was opened with
)

type Link struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Url         string `json:"url"`
	Group       string `json:"group,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

type Settings struct {
	// HostIP overrides the auto-detected server IP. Empty means auto-detect.
	HostIP string `json:"hostIp,omitempty"`
	// Title is the dashboard name shown in the header and the browser tab.
	Title string `json:"title,omitempty"`
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

func checkLen(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return invalid("%s is too long (max %d characters)", field, max)
	}
	return nil
}

// NormalizeLink trims and validates a link. The ID is left untouched.
func NormalizeLink(l Link) (Link, error) {
	l.Title = strings.TrimSpace(l.Title)
	l.Group = strings.TrimSpace(l.Group)
	l.Description = strings.TrimSpace(l.Description)
	l.Icon = strings.TrimSpace(l.Icon)

	if l.Title == "" {
		return l, invalid("title is required")
	}
	if err := checkLen("title", l.Title, 200); err != nil {
		return l, err
	}
	if err := checkLen("group", l.Group, 64); err != nil {
		return l, err
	}
	if err := checkLen("description", l.Description, 300); err != nil {
		return l, err
	}
	if err := checkLen("icon", l.Icon, 1024); err != nil {
		return l, err
	}

	u, err := NormalizeURL(l.Url)
	if err != nil {
		return l, err
	}
	l.Url = u
	return l, nil
}

// NormalizeURL validates a link URL and adds "http://" when no scheme is given,
// so "192.168.1.20:8096" becomes "http://192.168.1.20:8096".
func NormalizeURL(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", invalid("url is required")
	}
	if len(u) > 2048 {
		return "", invalid("url is too long")
	}
	for _, r := range u {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", invalid("url must not contain spaces")
		}
	}

	lower := strings.ToLower(u)
	for _, scheme := range []string{"javascript:", "data:", "vbscript:"} {
		if strings.HasPrefix(lower, scheme) {
			return "", invalid("%s URLs are not allowed", strings.TrimSuffix(scheme, ":"))
		}
	}
	if strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") {
		return u, nil
	}
	if !HasScheme(u) {
		u = "http://" + u
	}
	if SplitURL(u).Host == "" {
		return "", invalid("url must contain a host")
	}
	return u, nil
}

// ValidateHost checks a bare host or IP (no scheme, port or path).
func ValidateHost(host string, allowPlaceholders bool) error {
	if host == "" {
		return invalid("host is required")
	}
	if len(host) > 253 {
		return invalid("host is too long")
	}
	for _, r := range host {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return invalid("host must not contain spaces")
		}
	}
	if strings.ContainsAny(host, "/?#@") {
		return invalid("host must not contain a scheme, path or credentials")
	}
	if !allowPlaceholders && strings.ContainsAny(host, "{}") {
		return invalid("host must not contain placeholders")
	}
	return nil
}

func NormalizeSettings(s Settings) (Settings, error) {
	s.HostIP = strings.TrimSpace(s.HostIP)
	s.Title = strings.TrimSpace(s.Title)
	if s.HostIP != "" {
		if err := ValidateHost(s.HostIP, false); err != nil {
			return s, err
		}
	}
	if err := checkLen("title", s.Title, 80); err != nil {
		return s, err
	}
	return s, nil
}
