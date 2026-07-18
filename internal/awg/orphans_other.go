//go:build !linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

// killStrayAwgInterfaces is a no-op off Linux. AWG is a Linux kernel module;
// other platforms are not a supported deployment target for the AWG sidecar,
// so orphan sweeping is unnecessary there. There is no tun2socks daemon to
// sweep — routing into Xray is via an injected TUN inbound owned by Xray.
func killStrayAwgInterfaces() int { return 0 }
