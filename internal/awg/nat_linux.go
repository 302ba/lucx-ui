//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"os/exec"
)

// defaultRouteInterface returns the name of the interface holding the default
// route (the one that would carry outbound traffic to the internet). Used as
// the -o target for the MASQUERADE rule in PostUp. Returns empty when no
// default route exists (an unusual server, but we degrade gracefully: PostUp
// uses the rule but iptables will simply fail to match, which is logged but
// non-fatal).
func defaultRouteInterface() string {
	out, err := exec.CommandContext(context.Background(), "ip", "-o", "-4", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	return parseDefaultRouteInterface(string(out))
}
