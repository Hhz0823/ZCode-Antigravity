package auth

// protectedCredentialPrefix identifies current-user DPAPI values on Windows.
// Keeping the marker platform-neutral lets shared validation and tests inspect
// the serialized format without requiring the Windows implementation.
const protectedCredentialPrefix = "dpapi:v1:"
