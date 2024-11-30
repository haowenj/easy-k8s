package comm

import (
	"encoding/base64"
	"strings"
)

// Base64UrlEncode remove the base64 encoding of the special character '='
func Base64UrlEncode(text string) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(text)), "=")
}

// Base64UrlDecode decoding base64 encoding without '='
func Base64UrlDecode(input string) (string, error) {
	paddingNeeded := len(input) % 4
	if paddingNeeded > 0 {
		input += strings.Repeat("=", 4-paddingNeeded)
	}

	decodedBytes, err := base64.URLEncoding.DecodeString(input)
	if err != nil {
		return "", err
	}

	return string(decodedBytes), nil
}
