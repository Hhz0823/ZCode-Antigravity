//go:build !windows

package main

func automaticProxyCandidates() []proxyCandidate {
	return environmentProxyCandidates()
}
