package middleware

import (
	"net"
	"net/http"
	"strings"
)

func ExtractClientIP(r *http.Request, trusted []*net.IPNet) string {
	remoteIP := parseIPFromRemoteAddr(r.RemoteAddr)
	if remoteIP == nil {
		return ""
	}

	if isTrustedProxy(remoteIP, trusted) {
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			parts := strings.Split(forwarded, ",")
			for _, part := range parts {
				ip := net.ParseIP(strings.TrimSpace(part))
				if ip != nil {
					return ip.String()
				}
			}
		}
	}

	return remoteIP.String()
}

func parseIPFromRemoteAddr(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(remoteAddr)
}

func isTrustedProxy(ip net.IP, trusted []*net.IPNet) bool {
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
