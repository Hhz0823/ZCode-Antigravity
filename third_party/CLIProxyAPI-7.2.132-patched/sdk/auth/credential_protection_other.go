//go:build !windows && !darwin

package auth

const (
	credentialProtectionAvailable = false
	platformCredentialPrefix      = ""
)

func protectCredentialString(value string) (string, error)   { return value, nil }
func unprotectCredentialString(value string) (string, error) { return value, nil }
func credentialStringNeedsProtection(string) bool            { return false }
