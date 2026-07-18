//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// killStrayAwgInterfaces removes AWG kernel interfaces left over from a
// previous x-ui run and returns how many were removed. A survivor holds the
// inbound's UDP port with stale obfuscation, so new clients cannot connect.
// x-ui is the sole owner of awgN interfaces, so any "awg*" interface at
// startup is an orphan and is safe to delete. Routing of decrypted traffic
// into Xray is via an injected TUN inbound (no tun2socks daemon), so there
// are no userspace orphans to sweep — the TUN device is owned by Xray and
// dies with it.
func killStrayAwgInterfaces() int {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "awg") {
			continue
		}
		if err := exec.CommandContext(context.Background(), "ip", "link", "del", name).Run(); err == nil {
			killed++
		}
	}
	return killed
}

var (
	_ = strconv.Itoa
	_ = syscall.Kill
)
