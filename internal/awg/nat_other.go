//go:build !linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

// defaultRouteInterface is a no-op off Linux. AWG is a Linux kernel module;
// other platforms are not a supported deployment target for the AWG sidecar,
// so NAT setup is unnecessary there.
func defaultRouteInterface() string { return "" }
