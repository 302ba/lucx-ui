// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { formatInboundLabel } from '@/lib/inbounds/label';
import { preferPublicHost, resolveShareHost } from '@/lib/xray/inbound-link';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

export function isWireguardClient(client: ClientRecord | null | undefined): boolean {
  if (!client) return false;
  return !!(client.privateKey || client.publicKey || client.allowedIPs || client.preSharedKey || client.keepAlive);
}

export function findWireguardInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'wireguard');
}

export function buildWireguardClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
): string {
  const endpointHost = resolveShareHost(inbound ?? {}, inbound?.nodeAddress ?? '', preferPublicHost(host, publicHost));
  const address = client.allowedIPs || '10.0.0.2/32';
  const endpoint = `${endpointHost}:${inbound?.port || ''}`;
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email, client.comment].filter(Boolean).join(' - ');
  const lines = [
    '[Interface]',
    `PrivateKey = ${client.privateKey || client.password || ''}`,
    `Address = ${address}`,
    `DNS = ${inbound?.wgDns || '1.1.1.1, 1.0.0.1'}`,
  ];
  if (inbound?.wgMtu && inbound.wgMtu > 0) lines.push(`MTU = ${inbound.wgMtu}`);
  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${inbound?.wgPublicKey || ''}`);
  if (client.preSharedKey) lines.push(`PresharedKey = ${client.preSharedKey}`);
  lines.push('AllowedIPs = 0.0.0.0/0, ::/0', `Endpoint = ${endpoint}`);
  if (client.keepAlive && client.keepAlive > 0) lines.push(`PersistentKeepalive = ${client.keepAlive}`);
  return lines.join('\n');
}

// LUCX-HOOK: AWG — client .conf builder for AmneziaWG, mirroring buildWireguardClientConfig
// but inserting the Jc/Jmin/Jmax/S1-S4/H1-H4/I1-I5 obfuscation block into [Interface].
// AWG uses the same Curve25519 keypair/PSK/AllowedIPs as WireGuard, so the client
// record shape is identical; only the obfuscation lines (sourced from the inbound
// hints) are AWG-specific.

// isAwgClient reports whether the client carries AWG/WireGuard-style key fields.
// AWG clients use the same fields (privateKey/publicKey/allowedIPs/preSharedKey),
// so the same check applies — the distinction is made by the inbound protocol.
export function isAwgClient(client: ClientRecord | null | undefined): boolean {
  return isWireguardClient(client);
}

// findAwgInbound returns the first AWG inbound attached to the client.
export function findAwgInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'awg');
}

// buildAwgClientConfig builds a full AmneziaWG client .conf: [Interface] with
// the client's keypair, tunnel address, MTU, DNS, and the AWG obfuscation block
// (Jc/S1-S4/H1-H4/I1-I5), then [Peer] with the server public key, PSK, the
// full-tunnel AllowedIPs, and the endpoint.
export function buildAwgClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
): string {
  const endpointHost = resolveShareHost(inbound ?? {}, inbound?.nodeAddress ?? '', preferPublicHost(host, publicHost));
  const address = client.allowedIPs || '10.8.0.2/32';
  const endpoint = `${endpointHost}:${inbound?.port || ''}`;
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email, client.comment].filter(Boolean).join(' - ');
  const lines = [
    '[Interface]',
    `PrivateKey = ${client.privateKey || client.password || ''}`,
    `Address = ${address}`,
    `DNS = ${inbound?.wgDns || '1.1.1.1, 1.0.0.1'}`,
  ];
  if (inbound?.wgMtu && inbound.wgMtu > 0) lines.push(`MTU = ${inbound.wgMtu}`);
  // AWG obfuscation block (Jc/Jmin/Jmax/S1-S4/H1-H4/I1-I5) — pre-rendered by the
  // backend (inboundAwgHints) so the client .conf matches the server's .conf.
  if (inbound?.awgObfuscation) {
    lines.push(inbound.awgObfuscation.trimEnd());
  }
  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${inbound?.wgPublicKey || ''}`);
  if (client.preSharedKey) lines.push(`PresharedKey = ${client.preSharedKey}`);
  lines.push('AllowedIPs = 0.0.0.0/0, ::/0', `Endpoint = ${endpoint}`);
  if (client.keepAlive && client.keepAlive > 0) lines.push(`PersistentKeepalive = ${client.keepAlive}`);
  return lines.join('\n');
}
// END LUCX-HOOK
