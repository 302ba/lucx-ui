//go:build !linux

package awg

// defaultRouteInterface is a no-op off Linux. AWG is a Linux kernel module;
// other platforms are not a supported deployment target for the AWG sidecar,
// so NAT setup is unnecessary there.
func defaultRouteInterface() string { return "" }