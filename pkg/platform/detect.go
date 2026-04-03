package platform

import "runtime"

// ProxyHost returns the hostname nginx should use for proxy_pass.
// With ports mapping (not network_mode: host), the container uses
// host.docker.internal to reach services on the host.
func ProxyHost() string {
	return "host.docker.internal"
}

// IsMacOS returns true if running on macOS.
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}
