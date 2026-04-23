package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

func maskAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "ak_") {
		return "****"
	}
	if len(trimmed) <= 8 {
		return "****"
	}
	return trimmed[:6] + "..." + trimmed[len(trimmed)-4:]
}

func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func validateWebhookURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return &APIError{Code: "PAY-010", Message: "invalid webhook url"}
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return &APIError{Code: "PAY-010", Message: "invalid webhook url"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &APIError{Code: "PAY-010", Message: "invalid webhook url"}
	}
	if parsed.User != nil {
		return &APIError{Code: "PAY-010", Message: "invalid webhook url"}
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return &APIError{Code: "PAY-010", Message: "invalid webhook url"}
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".local") || strings.HasSuffix(lower, ".internal") {
		return &APIError{Code: "PAY-010", Message: "webhook url blocked"}
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if isPrivateOrLocalIP(ip) {
			return &APIError{Code: "PAY-010", Message: "webhook url blocked"}
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		addr, ok := netip.AddrFromSlice(ip)
		if ok && isPrivateOrLocalIP(addr) {
			return &APIError{Code: "PAY-010", Message: "webhook url blocked"}
		}
	}
	return nil
}

func isPrivateOrLocalIP(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}
