package main

import (
	"net"
	"testing"
)

func TestWindowsProxyParsingAndLoopbackGuard(t *testing.T) {
	candidates := parseWindowsProxyServer("http=127.0.0.1:10808;https=127.0.0.1:10808;socks=127.0.0.1:10809")
	if len(candidates) != 3 || candidates[0].Value != "http://127.0.0.1:10808" {
		t.Fatalf("candidates = %#v", candidates)
	}
	for _, value := range []string{"127.0.0.1:10808", "socks5://localhost:10808", "http://[::1]:10808"} {
		if _, ok := normalizeLoopbackProxy(value); !ok {
			t.Fatalf("loopback proxy rejected: %s", value)
		}
	}
	for _, value := range []string{"http://192.168.1.9:10808", "ftp://127.0.0.1:10808", "http://127.0.0.1"} {
		if _, ok := normalizeLoopbackProxy(value); ok {
			t.Fatalf("unsafe proxy accepted: %s", value)
		}
	}
}

func TestResolveProxyPrefersManualThenAutomaticLoopback(t *testing.T) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(key, "")
	}
	a := testApp(t)
	a.settings.ProxyURL = "socks5://127.0.0.1:19090"
	if value, source := a.resolveProxy(); value != a.settings.ProxyURL || source != "手动代理" {
		t.Fatalf("manual proxy = %q %q", value, source)
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	a.settings.ProxyURL = ""
	t.Setenv("HTTPS_PROXY", "http://"+listener.Addr().String())
	if value, source := a.resolveProxy(); value != "http://"+listener.Addr().String() || source != "环境代理" {
		t.Fatalf("automatic proxy = %q %q", value, source)
	}
}
