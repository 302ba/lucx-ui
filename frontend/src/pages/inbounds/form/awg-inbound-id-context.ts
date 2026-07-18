// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// LUCX-HOOK: AWG — exposes the editing inbound's DB id to the LucX protocol
// form (the diagnostics button needs it for /panel/api/inbounds/:id/awgDiagnostics)
// without threading props through upstream components between the modal and
// the protocol tab. null = new unsaved inbound (diagnostics unavailable).
import { createContext, useContext } from 'react';

const AwgInboundIdContext = createContext<number | null>(null);

export const AwgInboundIdProvider = AwgInboundIdContext.Provider;

export function useAwgInboundId(): number | null {
  return useContext(AwgInboundIdContext);
}
// END LUCX-HOOK
