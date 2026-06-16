package skills

import (
	"fmt"
	"net"
	"strings"
)

var unsafeHostnames = []string{
	"metadata.google.internal",
	"metadata.google.internal.",
	"metadata.gcp.internal",
	"metadata.azure.internal",
	"metadata.aws.internal",
}

var cgnatRanges = []*net.IPNet{
	mustCIDR("100.64.0.0/10"),
	mustCIDR("198.18.0.0/15"),
}

func mustCIDR(s string) *net.IPNet {
	_, c, err := net.ParseCIDR(s)
	if err != nil {
		panic("bad cidr: " + s)
	}
	return c
}

func hostLooksUnsafe(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || host == "localhost" {
		return true
	}
	if strings.HasPrefix(host, "[") {
		return true
	}
	for _, name := range unsafeHostnames {
		if host == name {
			return true
		}
	}
	h := host
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "]") {
		h = host[:i]
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return true
		}
		for _, cidr := range cgnatRanges {
			if cidr.Contains(ip) {
				return true
			}
		}
		return false
	}
	if addrs, err := net.LookupHost(host); err == nil {
		for _, addr := range addrs {
			if ip := net.ParseIP(addr); ip != nil {
				if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
					return true
				}
				for _, cidr := range cgnatRanges {
					if cidr.Contains(ip) {
						return true
					}
				}
			}
		}
	}
	return false
}

func strArg(in map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := in[k]
		if !ok || v == nil {
			continue
		}
		switch s := v.(type) {
		case string:
			return s
		default:
			return fmt.Sprint(s)
		}
	}
	return ""
}

func extractByPath(data any, key string) (any, error) {
	fields := strings.Split(key, ".")
	current := data
	for _, field := range fields {
		switch c := current.(type) {
		case map[string]any:
			v, ok := c[field]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", field)
			}
			current = v
		default:
			return nil, fmt.Errorf("not an object at %s", field)
		}
	}
	return current, nil
}
