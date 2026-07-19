// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AwgJob reconciles the running AWG kernel-interface sidecars against the
// enabled AWG inbounds in the database, restarts any that crashed, folds the
// per-inbound and per-client traffic scraped from `awg show dump` into the
// usual accounting, and reports online clients from fresh handshakes (AWG
// clients never pass through Xray's stats API, so without this they show
// offline forever). Mirrors MtprotoJob.
type AwgJob struct {
	inboundService service.InboundService
	clientService  service.ClientService
}

// NewAwgJob creates a new AWG reconcile/traffic job instance.
func NewAwgJob() *AwgJob {
	return new(AwgJob)
}

// Run reconciles desired AWG inbounds with running interfaces and records
// traffic deltas.
func (j *AwgJob) Run() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("awg job: get inbounds failed:", err)
		return
	}

	var desired []awg.Instance
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := awg.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
		}
	}

	mgr := awg.GetManager()
	mgr.Reconcile(desired)

	deltas, peerDeltas, onlineByTag := mgr.CollectTraffic()

	// Map peer public keys to panel clients (email) for per-client accounting
	// and online status. One DB read per AWG inbound per tick; AWG inbounds
	// are few and their client lists are small.
	emailsByTag := make(map[string]map[string]string, len(onlineByTag))
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		clients, err := j.clientService.ListForInbound(nil, ib.Id)
		if err != nil {
			logger.Warning("awg job: list clients for inbound", ib.Id, "failed:", err)
			continue
		}
		byKey := make(map[string]string, len(clients))
		for _, c := range clients {
			if c.Enable && c.PublicKey != "" {
				byKey[c.PublicKey] = c.Email
			}
		}
		emailsByTag[ib.Tag] = byKey
	}

	traffics := make([]*xray.Traffic, 0, len(deltas))
	for _, d := range deltas {
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       d.Tag,
			Up:        d.Up,
			Down:      d.Down,
		})
	}

	clientTraffics := make([]*xray.ClientTraffic, 0, len(peerDeltas))
	for _, pd := range peerDeltas {
		email, ok := emailsByTag[pd.Tag][pd.PublicKey]
		if !ok {
			continue
		}
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: email,
			Up:    pd.Up,
			Down:  pd.Down,
		})
	}

	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("awg job: add traffic failed:", err)
		}
	}

	// Online status: fresh handshake (<180 s) = online. activeTags marks the
	// running AWG inbounds so the "active inbound" gating works for AWG too.
	var onlineEmails []string
	for tag, keys := range onlineByTag {
		for _, key := range keys {
			if email, ok := emailsByTag[tag][key]; ok {
				onlineEmails = append(onlineEmails, email)
			}
		}
	}
	activeTags := make([]string, 0, len(desired))
	for _, inst := range desired {
		activeTags = append(activeTags, inst.Tag)
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}
