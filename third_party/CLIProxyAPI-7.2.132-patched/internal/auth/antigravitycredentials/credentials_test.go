package antigravitycredentials

import (
	"strings"
	"testing"
)

func TestLoadTrimsConfiguredValues(t *testing.T) {
	t.Setenv(ClientIDEnvironmentVariable, "  development-client-id  ")
	t.Setenv(ClientSecretEnvironmentVariable, "  development-client-secret  ")

	client, errLoad := Load()
	if errLoad != nil {
		t.Fatalf("Load: %v", errLoad)
	}
	if client.ID != "development-client-id" || client.Secret != "development-client-secret" {
		t.Fatalf("unexpected client: %#v", client)
	}
}

func TestLoadReportsEveryMissingVariable(t *testing.T) {
	t.Setenv(ClientIDEnvironmentVariable, "")
	t.Setenv(ClientSecretEnvironmentVariable, "")

	_, errLoad := Load()
	if errLoad == nil {
		t.Fatal("Load accepted missing OAuth client configuration")
	}
	message := errLoad.Error()
	for _, variable := range []string{ClientIDEnvironmentVariable, ClientSecretEnvironmentVariable} {
		if !strings.Contains(message, variable) {
			t.Fatalf("error %q does not mention %s", message, variable)
		}
	}
}
