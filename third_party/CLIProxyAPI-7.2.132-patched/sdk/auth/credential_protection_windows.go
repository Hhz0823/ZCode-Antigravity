//go:build windows

package auth

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectCredentialString(value string) (string, error) {
	if value == "" || len(value) >= len(protectedCredentialPrefix) && value[:len(protectedCredentialPrefix)] == protectedCredentialPrefix {
		return value, nil
	}
	in := bytesToDataBlob([]byte(value))
	var out windows.DataBlob
	if err := windows.CryptProtectData(in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return "", fmt.Errorf("DPAPI protect credential: %w", err)
	}
	defer func() {
		if out.Data != nil {
			_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
		}
	}()
	protected := unsafe.Slice(out.Data, out.Size)
	return protectedCredentialPrefix + base64.RawStdEncoding.EncodeToString(protected), nil
}

func unprotectCredentialString(value string) (string, error) {
	if len(value) < len(protectedCredentialPrefix) || value[:len(protectedCredentialPrefix)] != protectedCredentialPrefix {
		return value, nil
	}
	ciphertext, errDecode := base64.RawStdEncoding.DecodeString(value[len(protectedCredentialPrefix):])
	if errDecode != nil {
		return "", fmt.Errorf("DPAPI decode credential: %w", errDecode)
	}
	in := bytesToDataBlob(ciphertext)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return "", fmt.Errorf("DPAPI unprotect credential: %w", err)
	}
	defer func() {
		if out.Data != nil {
			_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
		}
	}()
	plain := unsafe.Slice(out.Data, out.Size)
	return string(plain), nil
}

func credentialStringNeedsProtection(value string) bool {
	return value != "" && !strings.HasPrefix(value, protectedCredentialPrefix)
}

func bytesToDataBlob(data []byte) *windows.DataBlob {
	blob := &windows.DataBlob{Size: uint32(len(data))}
	if len(data) > 0 {
		blob.Data = &data[0]
	}
	return blob
}
