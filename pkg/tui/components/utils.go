package components

import "strings"

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
