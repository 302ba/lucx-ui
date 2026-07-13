// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/awg/cps"
)

// awgGenerateObfuscationRequest is the body the AWG inbound form posts to
// /panel/api/inbounds/awg/generateObfuscation. obfProfile selects the
// junk/transport strength (lite/standard/pro); mimicryProfile picks the CPS
// packet shape (tls/dns/sip/quic); region selects the front-domain pool
// (ru/world); domain is an optional explicit front host (empty = random from
// the pool); fullI1I5 reports whether I1-I5 are all emitted (Pro) or just I1
// (Lite/Standard).
type awgGenerateObfuscationRequest struct {
	ObfProfile      string `json:"obfProfile"`
	MimicryProfile  string `json:"mimicryProfile"`
	Region          string `json:"region"`
	Domain          string `json:"domain"`
	FullI1I5        bool   `json:"fullI1I5"`
}

// awgGenerateObfuscation generates a fresh set of AmneziaWG obfuscation
// parameters (Jc/Jmin/Jmax/S1-S4/H1-H4) and CPS packets (I1-I5) for the AWG
// inbound form. The frontend calls this when the user clicks "generate
// obfuscation" so the panel — not the browser — owns the RNG and the
// invariant-enforcing logic.
//
// LUCX-HOOK: AWG obfuscation generator endpoint.
func (a *InboundController) awgGenerateObfuscation(c *gin.Context) {
	var req awgGenerateObfuscationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid request body", err)
		return
	}
	if req.ObfProfile == "" {
		req.ObfProfile = string(cps.ObfStandard)
	}
	if req.MimicryProfile == "" {
		req.MimicryProfile = string(cps.ProfileTLS)
	}
	if req.Region == "" {
		req.Region = string(cps.RegionWorld)
	}
	params, err := cps.GenerateAWGParams(cps.ObfProfile(req.ObfProfile))
	if err != nil {
		jsonMsg(c, "awg obfuscation: bad profile", err)
		return
	}
	cpsResult, err := cps.GenerateCPS(
		cps.MimicryProfile(req.MimicryProfile),
		cps.Region(req.Region),
		req.Domain,
		req.FullI1I5,
	)
	if err != nil {
		jsonMsg(c, "awg obfuscation: CPS generation failed", err)
		return
	}
	jsonObj(c, gin.H{
		"jc":  params.Jc,
		"jmin": params.Jmin,
		"jmax": params.Jmax,
		"s1":  params.S1,
		"s2":  params.S2,
		"s3":  params.S3,
		"s4":  params.S4,
		"h1":  params.H1,
		"h2":  params.H2,
		"h3":  params.H3,
		"h4":  params.H4,
		"i1":  cpsResult.I1,
		"i2":  cpsResult.I2,
		"i3":  cpsResult.I3,
		"i4":  cpsResult.I4,
		"i5":  cpsResult.I5,
	}, nil)
}
// END LUCX-HOOK