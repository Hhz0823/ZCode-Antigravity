//go:build windows

package main

import (
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func automaticProxyCandidates() []proxyCandidate {
	candidates := environmentProxyCandidates()
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		enabled, _, errEnabled := key.GetIntegerValue("ProxyEnable")
		server, _, errServer := key.GetStringValue("ProxyServer")
		if errEnabled == nil && enabled != 0 && errServer == nil {
			candidates = append(candidates, parseWindowsProxyServer(server)...)
		}
	}
	if v2rayNRunning() {
		candidates = append(candidates,
			proxyCandidate{Value: "socks5://127.0.0.1:10808", Source: "v2rayN 自动代理"},
			proxyCandidate{Value: "http://127.0.0.1:10809", Source: "v2rayN 自动代理（兼容旧版）"},
		)
	}
	return candidates
}

func v2rayNRunning() bool {
	cmd := exec.Command("tasklist.exe", "/FO", "CSV", "/NH")
	prepareChildProcess(cmd)
	output, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(output)), "v2rayn")
}
