//go:build !windows

package audit

import "golang.org/x/sys/unix"

const oNoFollow = unix.O_NOFOLLOW
