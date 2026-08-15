package antigravity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCECodes holds the verifier and S256 challenge for an OAuth authorization flow.
type PKCECodes struct {
	CodeVerifier  string
	CodeChallenge string
}

// GeneratePKCECodes creates an RFC 7636 verifier/challenge pair.
func GeneratePKCECodes() (*PKCECodes, error) {
	random := make([]byte, 64)
	if _, err := rand.Read(random); err != nil {
		return nil, fmt.Errorf("antigravity pkce: generate verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return &PKCECodes{
		CodeVerifier:  verifier,
		CodeChallenge: base64.RawURLEncoding.EncodeToString(digest[:]),
	}, nil
}
