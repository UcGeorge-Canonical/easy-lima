package environment

import "runtime"

func HostArch() string { return runtime.GOARCH }

func hostArch() string { return runtime.GOARCH }
