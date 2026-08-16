package main

import (
	"bytes"
	"errors"
	"os"
	"unicode/utf8"
)

var windowsPowerShellUTF8BOM = []byte{0xef, 0xbb, 0xbf}

// writeWindowsPowerShellScript makes a UTF-8 script unambiguous to Windows
// PowerShell 5.1. Without a BOM, it decodes the file using the active ANSI code
// page; on Chinese Windows this can corrupt quotes next to CJK text and cause
// ParserError: UnexpectedToken before the installer starts.
func writeWindowsPowerShellScript(path string, script []byte) error {
	if !utf8.Valid(script) {
		return errors.New("embedded installer script is not valid UTF-8")
	}
	if bytes.HasPrefix(script, windowsPowerShellUTF8BOM) {
		return os.WriteFile(path, script, 0o600)
	}
	encoded := make([]byte, 0, len(windowsPowerShellUTF8BOM)+len(script))
	encoded = append(encoded, windowsPowerShellUTF8BOM...)
	encoded = append(encoded, script...)
	return os.WriteFile(path, encoded, 0o600)
}
