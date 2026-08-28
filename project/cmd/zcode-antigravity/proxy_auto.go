package main

import (
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

type proxyCandidate struct {
	Value  string
	Source string
}

func (a *app) resolveProxy() (string, string) {
	if configured := strings.TrimSpace(a.currentSettings().ProxyURL); configured != "" {
		return configured, "手动代理"
	}
	for _, candidate := range automaticProxyCandidates() {
		proxyURL, ok := normalizeLoopbackProxy(candidate.Value)
		if ok && proxyReachable(proxyURL) {
			return proxyURL, candidate.Source
		}
	}
	return "", "直连网络"
}

func (a *app) activeProxyStatus() (string, string) {
	a.proxyMu.RLock()
	proxyURL, source := a.activeProxy, a.proxySource
	a.proxyMu.RUnlock()
	if proxyURL != "" || source != "" {
		return proxyURL, source
	}
	return a.resolveProxy()
}

func environmentProxyCandidates() []proxyCandidate {
	seen := map[string]bool{}
	var candidates []proxyCandidate
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" && !seen[value] {
			seen[value] = true
			candidates = append(candidates, proxyCandidate{Value: value, Source: "环境代理"})
		}
	}
	return candidates
}

func parseWindowsProxyServer(raw string) []proxyCandidate {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "=") {
		return []proxyCandidate{{Value: raw, Source: "Windows 系统代理"}}
	}
	values := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	var candidates []proxyCandidate
	for _, key := range []string{"https", "http", "socks", "socks5"} {
		if value := values[key]; value != "" {
			scheme := "http://"
			if strings.HasPrefix(key, "socks") {
				scheme = "socks5://"
			}
			candidates = append(candidates, proxyCandidate{Value: scheme + value, Source: "Windows 系统代理"})
		}
	}
	return candidates
}

func normalizeLoopbackProxy(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Port() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5") {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", false
	}
	return parsed.String(), true
}

func proxyReachable(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 350*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
