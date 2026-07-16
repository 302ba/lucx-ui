// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"math/rand"
	"strings"
	"testing"
)

func TestGenerateAWGParams_Invariants(t *testing.T) {
	SetRand(rand.New(rand.NewSource(42)))
	for _, prof := range []ObfProfile{ObfLite, ObfStandard, ObfPro} {
		for i := 0; i < 200; i++ {
			p, err := GenerateAWGParams(prof)
			if err != nil {
				t.Fatalf("profile %s iter %d: %v", prof, i, err)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("profile %s iter %d validate: %v", prof, i, err)
			}
			// H1-H4 must be "lo-hi" ranges, non-empty, and in disjoint quadrants.
			for _, h := range []string{p.H1, p.H2, p.H3, p.H4} {
				if !strings.Contains(h, "-") {
					t.Fatalf("profile %s: H range %q missing '-'", prof, h)
				}
			}
		}
	}
}

func TestGenerateCPS_AllProfilesNonEmpty(t *testing.T) {
	SetRand(rand.New(rand.NewSource(7)))
	for _, mp := range []MimicryProfile{ProfileTLS, ProfileDNS, ProfileSIP, ProfileQUIC} {
		for _, reg := range []Region{RegionRU, RegionWorld} {
			r1, err := GenerateCPS(mp, reg, "", true)
			if err != nil {
				t.Fatalf("profile %s region %s onlyI1: %v", mp, reg, err)
			}
			if r1.I1 == "" {
				t.Fatalf("profile %s region %s: I1 empty", mp, reg)
			}
			if r1.I2 != "" {
				t.Fatalf("profile %s region %s: onlyI1 leaked I2", mp, reg)
			}
			r5, err := GenerateCPS(mp, reg, "", false)
			if err != nil {
				t.Fatalf("profile %s region %s full: %v", mp, reg, err)
			}
			for i, v := range []string{r5.I1, r5.I2, r5.I3, r5.I4, r5.I5} {
				if v == "" {
					t.Fatalf("profile %s region %s: I%d empty in full mode", mp, reg, i+1)
				}
			}
		}
	}
}

func TestGenerateCPS_ExplicitDomain(t *testing.T) {
	SetRand(rand.New(rand.NewSource(1)))
	r, err := GenerateCPS(ProfileTLS, RegionWorld, "example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	if r.I1 == "" {
		t.Fatal("explicit domain produced empty I1")
	}
}

func TestGenerateCPS_DNSHasR2Prefix(t *testing.T) {
	rand.Seed(3)
	r, err := GenerateCPS(ProfileDNS, RegionWorld, "example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.I1, "<r 2>") {
		t.Fatalf("DNS packet must start with <r 2>, got %q", r.I1[:20])
	}
}

func TestGenerateCPS_NonDNSNoR2Prefix(t *testing.T) {
	rand.Seed(5)
	for _, mp := range []MimicryProfile{ProfileTLS, ProfileSIP, ProfileQUIC} {
		r, err := GenerateCPS(mp, RegionWorld, "example.com", true)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(r.I1, "<r 2>") {
			t.Fatalf("profile %s must not use <r 2> prefix", mp)
		}
	}
}

func TestDomainPool_NonEmpty(t *testing.T) {
	for _, mp := range []MimicryProfile{ProfileTLS, ProfileDNS, ProfileSIP, ProfileQUIC} {
		for _, reg := range []Region{RegionRU, RegionWorld} {
			pool := DomainPool(mp, reg)
			if len(pool) == 0 {
				t.Fatalf("pool empty for %s/%s", mp, reg)
			}
		}
	}
}