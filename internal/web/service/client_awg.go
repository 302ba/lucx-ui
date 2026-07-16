// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// defaultAwgBase is the tunnel subnet AWG clients are allocated from. It is
// intentionally distinct from WireGuard's 10.0.0.0/24 so an AWG inbound and a
// WireGuard inbound on the same panel don't collide on peer addresses.
const defaultAwgBase = "10.8.0.0/24"

// defaultAwgClients fills in blank AmneziaWG credentials for newly added
// clients, mirroring defaultWireguardClients: a generated Curve25519 keypair
// when none was provided, a derived public key when only a private key was
// given, a fresh PSK when none was provided, and a unique tunnel address
// allocated from the inbound's subnet. It mutates both the typed clients and
// the parallel raw client maps that get persisted into the inbound settings.
// Existing values are never overwritten, so editing a client never rotates its
// keys.
//
// AmneziaWG uses the same Curve25519 base keypair and PSK format as WireGuard;
// only the obfuscation parameters (Jc/S1-S4/H1-H4/I1-I5) are AWG-specific and
// live on the inbound (shared by all peers), not on the client.
func defaultAwgClients(existing, clients []model.Client, interfaceClients []any) error {
	used := make([]string, 0)
	for i := range existing {
		used = append(used, existing[i].AllowedIPs...)
	}
	base := wireguardAllocationBase(used, defaultAwgBase)
	for i := range clients {
		c := &clients[i]
		if c.PrivateKey == "" && c.PublicKey == "" {
			priv, pub, err := wgutil.GenerateWireguardKeypair()
			if err != nil {
				return err
			}
			c.PrivateKey = priv
			c.PublicKey = pub
		} else if c.PublicKey == "" && c.PrivateKey != "" {
			pub, err := wgutil.PublicKeyFromPrivate(c.PrivateKey)
			if err != nil {
				return err
			}
			c.PublicKey = pub
		}
		if c.PreSharedKey == "" {
			psk, err := wgutil.GenerateWireguardPSK()
			if err != nil {
				return err
			}
			c.PreSharedKey = psk
		}
		if len(c.AllowedIPs) == 0 {
			addr, err := allocateWireguardAddress(used, base)
			if err != nil {
				return err
			}
			c.AllowedIPs = []string{addr}
		} else {
			normalized, err := normalizeWireguardAllowedIPs(c.AllowedIPs)
			if err != nil {
				return err
			}
			if len(normalized) == 0 {
				return common.NewError("awg: allowedIPs has no usable entry")
			}
			if hit := wireguardAllowedIPsCollision(normalized, used); hit != "" {
				return common.NewError("awg: allowedIPs entry already used by another client:", hit)
			}
			c.AllowedIPs = normalized
		}
		used = append(used, c.AllowedIPs...)

		if i < len(interfaceClients) {
			if m, ok := interfaceClients[i].(map[string]any); ok {
				m["privateKey"] = c.PrivateKey
				m["publicKey"] = c.PublicKey
				m["allowedIPs"] = c.AllowedIPs
				if c.PreSharedKey != "" {
					m["preSharedKey"] = c.PreSharedKey
				}
				if c.KeepAlive > 0 {
					m["keepAlive"] = c.KeepAlive
				}
				interfaceClients[i] = m
			}
		}
	}
	return nil
}
