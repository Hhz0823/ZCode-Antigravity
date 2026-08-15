//go:build darwin

package auth

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func init() {
	testKey := bytes.Repeat([]byte{0x42}, 32)
	credentialEncryptionKey = func(bool) ([]byte, error) {
		return append([]byte(nil), testKey...), nil
	}
}

func TestDarwinCredentialProtectionRoundTrip(t *testing.T) {
	protected, errProtect := protectCredentialString("mac-secret-token")
	if errProtect != nil {
		t.Fatalf("protectCredentialString: %v", errProtect)
	}
	if !strings.HasPrefix(protected, keychainCredentialPrefix) || strings.Contains(protected, "mac-secret-token") {
		t.Fatalf("unexpected protected credential: %q", protected)
	}
	plain, errUnprotect := unprotectCredentialString(protected)
	if errUnprotect != nil {
		t.Fatalf("unprotectCredentialString: %v", errUnprotect)
	}
	if plain != "mac-secret-token" {
		t.Fatalf("unprotected credential = %q", plain)
	}
}

func TestDarwinCredentialProtectionRejectsTampering(t *testing.T) {
	protected, errProtect := protectCredentialString("mac-secret-token")
	if errProtect != nil {
		t.Fatal(errProtect)
	}
	payload, errDecode := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(protected, keychainCredentialPrefix))
	if errDecode != nil {
		t.Fatal(errDecode)
	}
	payload[len(payload)/2] ^= 0x01
	protected = keychainCredentialPrefix + base64.RawStdEncoding.EncodeToString(payload)
	if _, errUnprotect := unprotectCredentialString(protected); errUnprotect == nil {
		t.Fatal("tampered credential decrypted successfully")
	}
}
