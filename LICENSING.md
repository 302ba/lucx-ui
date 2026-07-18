# Licensing — LucX-UI

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) and uses **two licenses**, depending on which part of the code you are looking at.

## 1. Upstream 3x-ui code — GPL-3.0

All original 3x-ui code remains under the **GNU General Public License v3.0** (see [LICENSE](LICENSE)), as required by the upstream project. This includes every file not listed below, and every upstream file that carries inline `LUCX-HOOK` integration blocks (the surrounding file stays GPL; the hook blocks themselves are small integration glue).

## 2. LucX-specific components — PolyForm Noncommercial 1.0.0

The components **authored by the LucX-UI project** are licensed under the **PolyForm Noncommercial License 1.0.0** (see [LICENSE-PolyForm-Noncommercial.txt](LICENSE-PolyForm-Noncommercial.txt)):

- `internal/awg/` — the entire AWG sidecar (manager, process, instance, traffic, diagnostics, NAT/orphan helpers, `cps/`, `signature/`)
- `internal/lucx/` — Smart Cluster packages (`parser/`, `nodetype/`, `outbound_link/`)
- `internal/database/migrate_awg.go` and its test
- `internal/web/controller/awg.go` — AWG API endpoints
- `internal/web/job/awg_job.go` — AWG reconcile cron
- `internal/web/service/client_awg.go` — AWG client provisioning
- `frontend/src/schemas/protocols/inbound/awg.ts` — AWG Zod schema
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` — AWG form
- `frontend/src/pages/inbounds/form/awg-inbound-id-context.ts`
- `frontend/src/pages/clients/wireguardConfig.ts` — client `.conf` builder
- `bin/install-awg-module.sh` — DKMS kernel-module installer
- `bin/check-lucx.sh`, `bin/pre-push` — development scripts

Every such file carries an SPDX header:

```
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
```

**What this means in practice:**

- **Free** for personal, educational, research, charity, and government use — run as many panels as you like.
- **Commercial use requires permission.** Reselling VPN access (paid panels/subscriptions built on this code), offering it as a paid service, or embedding these components in a commercial product requires explicit written permission from the LucX-UI author.
- You **cannot** sublicense these components or strip the license headers.
- The GPL-3.0 obligations for the upstream 3x-ui code apply to the project as a whole regardless.

## Why the split?

3x-ui is GPL-3.0, and a fork cannot be relicensed as a whole. At the same time, the AWG sidecar and Smart Cluster are original work the author wants to keep non-commercial — usable and forkable by anyone for themselves, but not repackaged into a paid VPN business without a conversation first. Per-file SPDX headers make the boundary unambiguous; if a file has no SPDX header, it is upstream GPL-3.0.

## Contact

For commercial licensing, open an issue at <https://github.com/AlexeyLCP/lucx-ui/issues> or contact the repository owner.
