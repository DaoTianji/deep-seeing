// Package world is the World Gateway: safe fetch, search, Source provenance (P8).
package world

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// UntrustedFence markers wrap fetched external content so models treat it as data, not instructions.
const (
	UntrustedBegin = "<<<UNTRUSTED_EXTERNAL_CONTENT>>>"
	UntrustedEnd   = "<<<END_UNTRUSTED_EXTERNAL_CONTENT>>>"
)

// WrapUntrustedContent fences untrusted text for prompt injection resistance.
func WrapUntrustedContent(sourceURL, body string) string {
	src := strings.TrimSpace(sourceURL)
	if src == "" {
		src = "(unknown)"
	}
	var b strings.Builder
	b.WriteString(UntrustedBegin)
	b.WriteString("\nsource: ")
	b.WriteString(src)
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n")
	b.WriteString(UntrustedEnd)
	return b.String()
}

// ValidateFetchURL enforces P8 SSRF policy for future search_web / read_webpage.
// Rules: http/https only; no loopback / RFC1918 / link-local / metadata IPs;
// host is resolved and every IP re-checked.
func ValidateFetchURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("url required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed; only http/https", u.Scheme)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("host required")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || lower == "metadata.google.internal" {
		return fmt.Errorf("host %q blocked", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if err := denyIP(ip); err != nil {
			return err
		}
		return nil
	}
	dnsCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(dnsCtx, host)
	if err != nil {
		return fmt.Errorf("dns resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dns resolve %q: no addresses", host)
	}
	for _, addr := range ips {
		if err := denyIP(addr.IP); err != nil {
			return fmt.Errorf("resolved %s: %w", addr.IP, err)
		}
	}
	return nil
}

// ValidateRedirectURL re-validates a redirect target under the same SSRF rules.
func ValidateRedirectURL(raw string) error {
	return ValidateFetchURL(raw)
}

func denyIP(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("nil ip")
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("ip %s blocked (loopback/private/link-local/multicast)", ip)
	}
	// Cloud metadata common address
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return fmt.Errorf("ip %s blocked (link-local/metadata)", ip)
		}
	}
	return nil
}
