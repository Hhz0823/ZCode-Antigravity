// Package antigravitycredentials loads the OAuth client configuration used by
// the Antigravity provider. The public source tree intentionally contains no
// embedded OAuth client identity or secret.
package antigravitycredentials

import (
	"fmt"
	"os"
	"strings"
)

const (
	// ClientIDEnvironmentVariable is the environment variable containing the
	// OAuth desktop client ID used for Antigravity authorization.
	ClientIDEnvironmentVariable = "ANTIGRAVITY_OAUTH_CLIENT_ID"
	// ClientSecretEnvironmentVariable is the environment variable containing
	// the matching OAuth desktop client secret.
	ClientSecretEnvironmentVariable = "ANTIGRAVITY_OAUTH_CLIENT_SECRET"
)

// OAuthClient contains the configured OAuth desktop client credentials.
type OAuthClient struct {
	ID     string
	Secret string
}

// Release maintainers may inject these values at link time with -X. Public
// source builds leave both empty and continue to require the environment.
var (
	embeddedClientID     string
	embeddedClientSecret string
)

// Load reads and validates the Antigravity OAuth client configuration.
func Load() (OAuthClient, error) {
	client := OAuthClient{
		ID:     strings.TrimSpace(os.Getenv(ClientIDEnvironmentVariable)),
		Secret: strings.TrimSpace(os.Getenv(ClientSecretEnvironmentVariable)),
	}
	if client.ID == "" {
		client.ID = strings.TrimSpace(embeddedClientID)
	}
	if client.Secret == "" {
		client.Secret = strings.TrimSpace(embeddedClientSecret)
	}

	missing := make([]string, 0, 2)
	if client.ID == "" {
		missing = append(missing, ClientIDEnvironmentVariable)
	}
	if client.Secret == "" {
		missing = append(missing, ClientSecretEnvironmentVariable)
	}
	if len(missing) > 0 {
		return OAuthClient{}, fmt.Errorf("antigravity OAuth client is not configured; set %s", strings.Join(missing, " and "))
	}

	return client, nil
}
