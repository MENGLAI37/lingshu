//go:build windows

package audit

// O_NOFOLLOW is not available on Windows, use 0 to disable the flag.
const oNoFollow = 0
