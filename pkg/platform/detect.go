package platform

import "runtime"

// ProxyHost returns the hostname nginx should use for proxy_pass.
// Linux with network_mode: host can reach localhost directly.
// macOS Docker Desktop needs host.docker.internal.
func ProxyHost() string {
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}
	return "localhost"
}

// IsMacOS returns true if running on macOS.
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}
