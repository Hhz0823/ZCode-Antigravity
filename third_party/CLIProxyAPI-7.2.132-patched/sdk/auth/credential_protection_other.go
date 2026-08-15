//go:build !windows

package auth

func protectCredentialString(value string) (string, error)   { return value, nil }
func unprotectCredentialString(value string) (string, error) { return value, nil }
func credentialStringNeedsProtection(string) bool            { return false }
