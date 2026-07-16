// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

// rng is the package-level random source. In Go 1.20+ the global rand is
// automatically seeded, but tests need deterministic output. rand.Seed is
// deprecated; instead tests call SetRand with a seeded source.
var rng = rand.New(rand.NewSource(1))

// SetRand replaces the package-level random source. Used by tests for
// deterministic output; production code leaves the auto-seeded source.
func SetRand(r *rand.Rand) { rng = r }

// AWGParams are the junk/transport obfuscation parameters written into the
// awg-quick .conf [Interface] section. Jc/Jmin/Jmax control junk-packet
// insertion ahead of the handshake; S1-S4 are transport padding sizes; H1-H4
// are msgType replacement ranges. All fields follow the AmneziaWG spec
// invariants (Jmin < Jmax, |S1+56 − S2| ≥ 10, H1-H4 in disjoint quadrants).
type AWGParams struct {
	Jc   int
	Jmin int
	Jmax int
	S1   int
	S2   int
	S3   int
	S4   int
	H1   string // "lo-hi"
	H2   string
	H3   string
	H4   string
}

// randInt returns a random int in [lo, hi] inclusive. lo must be <= hi.
func randInt(lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + rng.Intn(hi-lo+1)
}

// Profile ranges, ported from pumbaX/awg-multi-script gen_awg_params.
// Lite = light obfuscation, Standard = medium, Pro = heavy.
type profileRanges struct {
	jcmin, jcmax   int
	jminLo, jminHi int
	jmaxLo, jmaxHi int
	s1Lo, s1Hi     int
	s2Lo, s2Hi     int
	s3Lo, s3Hi     int
	s4Lo, s4Hi     int
}

func rangesFor(p ObfProfile) (profileRanges, error) {
	switch p {
	case ObfLite:
		return profileRanges{3, 5, 5, 15, 45, 55, 97, 107, 17, 27, 16, 26, 4, 10}, nil
	case ObfStandard:
		return profileRanges{5, 8, 30, 80, 100, 250, 30, 80, 30, 80, 15, 32, 10, 20}, nil
	case ObfPro:
		return profileRanges{4, 16, 50, 256, 300, 1000, 15, 150, 15, 150, 8, 64, 6, 31}, nil
	default:
		return profileRanges{}, errors.New("awg params: unknown profile")
	}
}

// quadrant returns the [lo, hi] bounds for H-index n (0..3). The full H
// range [5, 2^31-1] is split into 4 disjoint quadrants of 2^29 each so H1-H4
// never overlap (matching the AmneziaWG recommended ranges). The upper bound
// is 2^31-1 for Windows-client compatibility.
func quadrant(n int) (lo, hi int) {
	const (
		hMin = 5
		hMax = 2147483647 // 2^31 - 1
		span = 1 << 29    // 2^29
	)
	lo = hMin + n*span
	hi = lo + span - 1
	if n == 3 {
		hi = hMax
	}
	return lo, hi
}

// genHRange returns one "lo-hi" string for quadrant n, with a width >= 1000
// (the AmneziaWG minimum recommended span). Mirrors pumbaX _gen_quadrant_pair.
func genHRange(n int) string {
	qmin, qmax := quadrant(n)
	span := qmax - qmin + 1
	lo := qmin + randInt(0, span/3)
	hi := qmin + 2*span/3 + randInt(0, span/3)
	if hi-lo < 1000 {
		hi = lo + 1000 + randInt(0, 9999)
		if hi > qmax {
			hi = qmax
		}
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}

// GenerateAWGParams produces a fresh set of junk/transport obfuscation
// parameters for the given strength profile. It enforces the AmneziaWG
// invariants: Jmin < Jmax (fixed by lifting Jmax), and |S1+56 − S2| ≥ 10
// (retry S2 up to 10 times, then shift it). H1-H4 are each in their own
// quadrant, so they never collide.
func GenerateAWGParams(profile ObfProfile) (AWGParams, error) {
	r, err := rangesFor(profile)
	if err != nil {
		return AWGParams{}, err
	}
	jc := randInt(r.jcmin, r.jcmax)
	jmin := randInt(r.jminLo, r.jminHi)
	jmax := randInt(r.jmaxLo, r.jmaxHi)
	if jmax <= jmin {
		jmax = jmin + randInt(100, 500)
	}
	s1 := randInt(r.s1Lo, r.s1Hi)
	s2 := randInt(r.s2Lo, r.s2Hi)
	// AmneziaWG requires S1 + 56 != S2; pumbaX strengthens this to a >=10 gap.
	for i := 0; i < 10 && abs((s1+56)-s2) < 10; i++ {
		s2 = randInt(r.s2Lo, r.s2Hi)
	}
	if abs((s1+56)-s2) < 10 {
		if s2 >= s1+56 {
			s2 += 10
		} else {
			s2 -= 10
		}
	}
	s3 := randInt(r.s3Lo, r.s3Hi)
	s4 := randInt(r.s4Lo, r.s4Hi)
	return AWGParams{
		Jc:   jc,
		Jmin: jmin,
		Jmax: jmax,
		S1:   s1,
		S2:   s2,
		S3:   s3,
		S4:   s4,
		H1:   genHRange(0),
		H2:   genHRange(1),
		H3:   genHRange(2),
		H4:   genHRange(3),
	}, nil
}

// AsConfLines returns the AWGParams as awg-quick .conf lines (Jc = …, H1 = lo-hi,
// …) suitable for the [Interface] section. The fields are emitted in the
// canonical AmneziaWG order.
func (p AWGParams) AsConfLines() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Jc = %d\n", p.Jc)
	fmt.Fprintf(&b, "Jmin = %d\n", p.Jmin)
	fmt.Fprintf(&b, "Jmax = %d\n", p.Jmax)
	fmt.Fprintf(&b, "S1 = %d\n", p.S1)
	fmt.Fprintf(&b, "S2 = %d\n", p.S2)
	fmt.Fprintf(&b, "S3 = %d\n", p.S3)
	fmt.Fprintf(&b, "S4 = %d\n", p.S4)
	fmt.Fprintf(&b, "H1 = %s\n", p.H1)
	fmt.Fprintf(&b, "H2 = %s\n", p.H2)
	fmt.Fprintf(&b, "H3 = %s\n", p.H3)
	fmt.Fprintf(&b, "H4 = %s\n", p.H4)
	return b.String()
}

// Validate enforces the AmneziaWG invariants on already-stored params (for
// migration / self-check). Returns nil when valid.
func (p AWGParams) Validate() error {
	if p.Jmax <= p.Jmin {
		return fmt.Errorf("awg: Jmax (%d) must be > Jmin (%d)", p.Jmax, p.Jmin)
	}
	if abs((p.S1+56)-p.S2) < 10 {
		return fmt.Errorf("awg: |S1+56 − S2| must be >= 10 (S1=%d S2=%d)", p.S1, p.S2)
	}
	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
