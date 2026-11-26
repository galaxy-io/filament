package httpapi

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// checkResourceBaseURL guards a rendered resource host before it receives the
// connection's authentication. The host must use HTTPS and belong to the same
// site as the connection base URL: the same host, or the same registrable
// domain under the public suffix list (data.mixpanel.com next to
// mixpanel.com). Loopback hosts may use plain HTTP when the connection host is
// loopback too, which keeps local test servers working.
func checkResourceBaseURL(resource, rendered, connectionBase string) error {
	u, err := url.Parse(rendered)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("base_url for resource %s must be an absolute URL without userinfo or fragment", resource)
	}
	conn, err := url.Parse(connectionBase)
	if err != nil || conn.Hostname() == "" {
		return fmt.Errorf("base_url for resource %s needs an absolute connection base URL to compare against", resource)
	}
	host, connHost := strings.ToLower(u.Hostname()), strings.ToLower(conn.Hostname())
	loopback := isLoopback(host) && isLoopback(connHost)
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return fmt.Errorf("base_url for resource %s must use https", resource)
	}
	if host == connHost || loopback || sameSite(host, connHost) {
		return nil
	}
	return fmt.Errorf("base_url host %s for resource %s is not on the connection's domain %s", host, resource, connHost)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameSite reports whether two host names share a registrable domain. IP
// addresses and public suffixes have none, so they only match themselves.
func sameSite(a, b string) bool {
	if net.ParseIP(a) != nil || net.ParseIP(b) != nil {
		return false
	}
	siteA, err := publicsuffix.EffectiveTLDPlusOne(a)
	if err != nil {
		return false
	}
	siteB, err := publicsuffix.EffectiveTLDPlusOne(b)
	return err == nil && siteA == siteB
}
