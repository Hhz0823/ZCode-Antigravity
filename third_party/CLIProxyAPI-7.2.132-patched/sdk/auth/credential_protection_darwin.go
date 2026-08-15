//go:build darwin

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const (
	keychainCredentialPrefix      = "keychain:v1:"
	keychainService               = "io.github.hhz0823.zcode-antigravity"
	keychainAccount               = "credential-master-key"
	credentialProtectionAvailable = true
	platformCredentialPrefix      = keychainCredentialPrefix
)

var credentialEncryptionKey = keychainMasterKey

func protectCredentialString(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, keychainCredentialPrefix) {
		return value, nil
	}
	key, errKey := credentialEncryptionKey(true)
	if errKey != nil {
		return "", errKey
	}
	gcm, errGCM := credentialGCM(key)
	if errGCM != nil {
		return "", errGCM
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, errRandom := rand.Read(nonce); errRandom != nil {
		return "", fmt.Errorf("generate Keychain credential nonce: %w", errRandom)
	}
	payload := gcm.Seal(nonce, nonce, []byte(value), []byte(keychainService))
	return keychainCredentialPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func unprotectCredentialString(value string) (string, error) {
	if !strings.HasPrefix(value, keychainCredentialPrefix) {
		return value, nil
	}
	payload, errDecode := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, keychainCredentialPrefix))
	if errDecode != nil {
		return "", fmt.Errorf("decode Keychain credential: %w", errDecode)
	}
	key, errKey := credentialEncryptionKey(false)
	if errKey != nil {
		return "", errKey
	}
	gcm, errGCM := credentialGCM(key)
	if errGCM != nil {
		return "", errGCM
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("Keychain credential payload is truncated")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, errOpen := gcm.Open(nil, nonce, ciphertext, []byte(keychainService))
	if errOpen != nil {
		return "", fmt.Errorf("decrypt Keychain credential: %w", errOpen)
	}
	return string(plain), nil
}

func credentialStringNeedsProtection(value string) bool {
	return value != "" && !strings.HasPrefix(value, keychainCredentialPrefix)
}

func credentialGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("Keychain credential key has invalid length %d", len(key))
	}
	block, errCipher := aes.NewCipher(key)
	if errCipher != nil {
		return nil, errCipher
	}
	return cipher.NewGCM(block)
}

func keychainMasterKey(create bool) ([]byte, error) {
	find := exec.Command("/usr/bin/security", "find-generic-password", "-w", "-s", keychainService, "-a", keychainAccount)
	stored, errFind := find.CombinedOutput()
	if errFind == nil {
		return decodeKeychainMasterKey(stored)
	}
	if !create {
		return nil, fmt.Errorf("read macOS Keychain credential key: %w", errFind)
	}
	var exitErr *exec.ExitError
	if !errors.As(errFind, &exitErr) || exitErr.ExitCode() != 44 {
		return nil, fmt.Errorf("read macOS Keychain credential key: %w", errFind)
	}

	key := make([]byte, 32)
	if _, errRandom := rand.Read(key); errRandom != nil {
		return nil, fmt.Errorf("generate macOS Keychain credential key: %w", errRandom)
	}
	encoded := base64.RawStdEncoding.EncodeToString(key)
	add := exec.Command("/usr/bin/security", "add-generic-password", "-U", "-s", keychainService, "-a", keychainAccount, "-w", encoded)
	if output, errAdd := add.CombinedOutput(); errAdd != nil {
		return nil, fmt.Errorf("store macOS Keychain credential key: %w (%s)", errAdd, strings.TrimSpace(string(output)))
	}
	return key, nil
}

func decodeKeychainMasterKey(stored []byte) ([]byte, error) {
	key, errDecode := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(stored)))
	if errDecode != nil {
		return nil, fmt.Errorf("decode macOS Keychain credential key: %w", errDecode)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("macOS Keychain credential key has invalid length %d", len(key))
	}
	return key, nil
}
