//go:build darwin

package environment

import "golang.org/x/sys/unix"

func hostMemory() (uint64, error) { return unix.SysctlUint64("hw.memsize") }
