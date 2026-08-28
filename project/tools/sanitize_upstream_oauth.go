package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var upstreamOAuthFiles = []string{
	"internal/api/handlers/management/api_tools.go",
	"internal/auth/antigravity/constants.go",
	"internal/runtime/executor/antigravity_executor.go",
}

var upstreamOAuthReplacements = []struct {
	pattern     *regexp.Regexp
	replacement []byte
}{
	{
		pattern:     regexp.MustCompile(`[0-9]{6,}-[A-Za-z0-9_-]+\.apps\.googleusercontent\.com`),
		replacement: []byte("ZCODE_REDACTED_GOOGLE_OAUTH_CLIENT_ID"),
	},
	{
		pattern:     regexp.MustCompile(`GOCSPX-[A-Za-z0-9_-]{20,}`),
		replacement: []byte("ZCODE_REDACTED_GOOGLE_OAUTH_CLIENT_SECRET"),
	},
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run sanitize_upstream_oauth.go <CLIProxyAPI-v7.2.132-directory>")
		os.Exit(2)
	}
	replacements, err := sanitizeUpstreamOAuth(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("sanitized upstream OAuth literals: %d\n", replacements)
}

func sanitizeUpstreamOAuth(root string) (int, error) {
	total := 0
	for _, relative := range upstreamOAuthFiles {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			return total, fmt.Errorf("inspect %s: %w", relative, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return total, fmt.Errorf("read %s: %w", relative, err)
		}
		updated := raw
		for _, rule := range upstreamOAuthReplacements {
			matches := rule.pattern.FindAllIndex(updated, -1)
			total += len(matches)
			updated = rule.pattern.ReplaceAllLiteral(updated, rule.replacement)
		}
		if string(updated) != string(raw) {
			if err := os.WriteFile(path, updated, info.Mode().Perm()); err != nil {
				return total, fmt.Errorf("write %s: %w", relative, err)
			}
		}
		for _, rule := range upstreamOAuthReplacements {
			if rule.pattern.Match(updated) {
				return total, fmt.Errorf("OAuth literal remained in %s", relative)
			}
		}
	}
	return total, nil
}
