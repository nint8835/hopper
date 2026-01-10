package utils

import "tailscale.com/util/truncate"

func PtrTo[T any](v T) *T {
	return &v
}

func TruncateString(s string, maxLen int) string {
	truncatedString := truncate.String(s, maxLen)

	if truncatedString != s {
		truncatedString = truncate.String(s, maxLen-3) + "..."
	}

	return truncatedString
}
