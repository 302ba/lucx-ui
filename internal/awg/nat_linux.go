//go:build linux

package awg

import (
	"context"
	"os/exec"
	"strings"
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
	fields := strings.Fields(strings.TrimSpace(string(out)))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}
