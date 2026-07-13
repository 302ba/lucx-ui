# LucX-UI — Agent Operating Manual

This file is the law for every agent working on this project. Read it completely before touching any code.

---

## Project Overview

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (currently **v3.5.0**) that adds native AmneziaWG (AWG) support as a kernel-interface sidecar, mirroring upstream's MTProto (mtg) sidecar architecture. LucX-specific code lives in `internal/awg/` and `internal/lucx/`; all integration points in upstream files are wrapped in `LUCX-HOOK` / `END LUCX-HOOK` markers.

**Upstream sync strategy:** migrate (not rebase). Each upstream release = fresh checkout of `origin/main` + port LucX code on top via LUCX-HOOK markers. The old `.patch`-file system is gone; integration is inline.

**Remotes:**
- `origin` → `MHSanaei/3x-ui` (upstream)
- `gh` → `AlexeyLCP/lucx-ui` (our fork)

**Active branch:** `feat/awg-sidecar-v3.5.0` (migrated from `feat/awg-sidecar` on v3.3.1).

---

## Core Philosophy

**Minimal invasion for easy upstream sync.** The goal is: every upstream release should be a near-trivial port. This means:
- LucX code lives in isolated packages (`internal/awg/`, `internal/lucx/`), not scattered across upstream files.
- Upstream files get ONLY `LUCX-HOOK` blocks — never free-form edits.
- The AWG sidecar should be as thin as the MTProto sidecar. If mtproto does it in N files, AWG should aim for N too. (See Known Issue #1 — we're currently at 19 vs 9.)

**AWG sidecar = mtproto pattern.** AWG runs as a kernel-interface sidecar exactly symmetric with `internal/mtproto/`:

```
mtproto:  mtg sidecar (userspace)  → TCP → SOCKS loopback inbound → Xray routing
AWG:      awg kernel module        → IP   → TUN inbound             → Xray routing
```

---

## Workflow: How an Agent Executes a Task

```
1. READ    → Read AGENTS.md, progress.md, git log --oneline -15, check latest state
2. AUDIT   → Read all relevant files, trace data flow end-to-end
3. PLAN    → Write a short plan: which files, what changes, what tests
4. BRANCH  → Work on the active migration branch (currently feat/awg-sidecar-v3.5.0)
5. CODE    → Implement changes inside LUCX-HOOK blocks in upstream files;
             new code goes in internal/awg/ or internal/lucx/
6. TEST    → Run tests:
               go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v
               cd frontend && npm run typecheck && npm run lint
7. BUILD   → Frontend: cd frontend && npm run build
             Backend:  go build -o /tmp/x-ui .
                       (requires frontend/dist to exist for //go:embed)
8. DEPLOY  → SCP to vps_finland_lucx, restart x-ui.service
9. VERIFY  → Check `sudo systemctl status x-ui`, check server logs
10. COMMIT → `git add` specific files, `git commit` with descriptive message (Russian)
11. STATUS → Output `git status` and `git log --oneline -15` after commits
12. DOCS   → Update progress.md and this file if architecture changes
```

---

## The 10 Rules

### 1. LUCX-HOOK Isolation

ALL changes to upstream 3x-ui files go inside `// LUCX-HOOK` / `// END LUCX-HOOK` markers. Never modify 3x-ui core code outside these markers without explicit instruction.

```go
// LUCX-HOOK: Description of what this does
// ... your code ...
// END LUCX-HOOK
```

```ts
// LUCX-HOOK: Description
// ... your code ...
// END LUCX-HOOK
```

Run `grep -rn "LUCX-HOOK" internal/ frontend/ install.sh` to find all integration points.

### 2. Isolated Modules

New functionality lives ONLY in:
- **Go:** `internal/awg/` — AWG sidecar (manager, process, instance, traffic, orphans)
- **Go:** `internal/lucx/` — subdirectories: `parser/`, `nodetype/`, `outbound_link/` (Smart Cluster)
- **Go:** `internal/database/migrate_awg.go` — legacy DB migration
- **Frontend:** `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema
- **Frontend:** `frontend/src/pages/inbounds/form/protocols/awg.tsx` — React form
- **Shell:** `bin/install-awg-module.sh` — DKMS install

Integration points (`model.go`, `db.go`, `web.go`, `runtime/local.go`, `service/xray.go`, `install.sh`, `inbound-defaults.ts`, `InboundFormModal.tsx`, `protocols/index.ts`, `primitives/protocol.ts`, `protocols/inbound/index.ts`) get LUCX-HOOK blocks only.

### 3. AWG Sidecar Architecture (mirrors mtproto)

AWG runs as a kernel-interface sidecar managed by `internal/awg.Manager`, exactly symmetric with `internal/mtproto.Manager`:

- **Manager** (`internal/awg/manager.go`): singleton with `Ensure`/`Reconcile`/`StopAll`/`CollectTraffic`/`SyncPeers`, fingerprint-based restart on config change, orphan sweep at first call.
- **Process** (`internal/awg/process.go`): wraps `awg-quick up/down` (kernel interface lifecycle, not a daemon). No tun2socks — routing is via Xray TUN inbound.
- **Instance** (`internal/awg/instance.go`): desired runtime state + `InstanceFromInbound` + `fingerprint`.
- **Traffic** (`internal/awg/traffic.go`): `awg show <iface> transfer` parsing for per-peer byte accounting (replaces mtg's Prometheus HTTP scrape).
- **Orphans** (`internal/awg/orphans_{linux,other}.go`): sweep orphaned awg interfaces from a previous x-ui run.
- **Job** (`internal/web/job/awg_job.go`): cron `@every 10s` — Reconcile desired inbounds + fold traffic deltas.
- **Egress** (`internal/web/service/xray.go:injectAwgEgress`): inject TUN inbound into generated Xray config when `routeThroughXray` is set, symmetric with `injectMtprotoEgress`.
- **Runtime** (`internal/web/runtime/local.go`): delegate AWG `AddInbound`/`DelInbound` to `awg.GetManager()`; `AddUser`/`RemoveUser` are no-ops (peer sync via Reconcile).

### 4. Paranoid Logging

Every critical operation logs with a prefix:
```
[LUCX-AWG]            — AWG service operations (legacy logAWG helper)
awg: <label> | <line> — sidecar process output (procLogWriter, matches mtproto)
```

### 5. No Telemt

The old LucX-UI had a `internal/lucx/telemt/` package for MTProto. Upstream replaced it with native `internal/mtproto/`. Do not re-add Telemt code; use the upstream MTProto implementation.

### 6. No tun2socks

The old architecture used a `tun2socks` userspace daemon to bridge the AWG kernel TUN to a hidden SOCKS5 inbound. The sidecar architecture makes it redundant — Xray supports a native TUN inbound (`injectAwgEgress`). Do not re-add tun2socks.

### 7. Test Discipline

- **Go:** `go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v`
- **Frontend:** `cd frontend && npm run typecheck && npm run lint`
- DB-dependent service tests require `CGO_ENABLED=1` (sqlite). Unit tests for AWG logic (instance, manager state, inject, stripHiddenKeys) run without cgo.
- Add tests for every new AWG function: instance parsing, fingerprint stability, config rendering, inject behavior, migration logic.

### 8. Upstream Sync

When pulling from upstream (`git fetch origin`):
- Re-run `go build ./internal/awg/... ./internal/lucx/...` — these packages have no upstream dependencies and should always compile.
- Check `grep -rn "LUCX-HOOK"` integration points for conflicts.
- Run `go test ./internal/awg/... ./internal/lucx/...` and frontend `typecheck`/`lint`.
- The migration procedure: fresh branch from `origin/main` → `git checkout <old-branch> -- <isolated-lucx-files>` → manually re-apply LUCX-HOOK blocks to changed upstream files (all upstream files with HOOK markers likely changed between releases).

### 9. Frontend Stack

Upstream rewrote the frontend from Vue to React + TypeScript + AntD v6 + Zod. AWG follows the same pattern:
- `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema (`AwgInboundSettingsSchema`)
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` — AntD form (`AwgFields`)
- `frontend/src/lib/xray/inbound-defaults.ts` — `createDefaultAwgInboundSettings`
- Registered in `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`

### 10. License

LucX-UI components (`internal/awg/`, `internal/lucx/`, `internal/database/migrate_awg.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `bin/install-awg-module.sh`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use (including VPN resale) requires explicit written permission from the author.

Original 3x-ui code remains under GPL-3.0.

---

## Architecture Map

```
internal/awg/                      AWG sidecar (mirrors internal/mtproto/)
├── manager.go                     Manager singleton: Ensure/Reconcile/StopAll/CollectTraffic/SyncPeers + renderServerConf/writeServerConfigFile
├── process.go                     Process wrapping awg-quick up/down + procLogWriter + awgConfigDir + awgQuick
├── instance.go                    Instance + InstanceFromInbound + fingerprint + PeerSpec
├── traffic.go                     scrapeTransfer via `awg show transfer` + Traffic type
├── orphans_linux.go               killStrayAwgInterfaces
├── orphans_other.go               no-op off Linux
├── instance_test.go               Instance/fingerprint/render tests
└── manager_test.go                Manager state-machine tests

internal/lucx/                     Smart Cluster
├── parser/                        SSH output → NodeCreds
├── nodetype/                      LucX vs vanilla detection (MTProtoVersion)
└── outbound_link/                 Inbound → outbound config generator

internal/database/
├── migrate_awg.go                 pruneLegacyAwgHiddenChildren + stripHiddenKeys
└── migrate_awg_test.go            stripHiddenKeys unit tests

internal/web/
├── runtime/local.go               AWG delegation in AddInbound/DelInbound (LUCX-HOOK)
├── job/awg_job.go                 AwgJob cron — Reconcile + CollectTraffic
├── service/xray.go                injectAwgEgress + AWG exclusion from Xray config (LUCX-HOOK)
└── web.go                         cadenceAwg + StopAll wiring (LUCX-HOOK)

internal/database/model/model.go   AWG Protocol const + validate oneof (LUCX-HOOK)
internal/database/db.go            pruneLegacyAwgHiddenChildren call (LUCX-HOOK)

frontend/src/
├── schemas/protocols/inbound/awg.ts        AwgInboundSettingsSchema (Zod)
├── pages/inbounds/form/protocols/awg.tsx   AwgFields (React + AntD)
├── lib/xray/inbound-defaults.ts            createDefaultAwgInboundSettings (LUCX-HOOK)
├── schemas/protocols/inbound/index.ts      InboundSettingsSchema union (LUCX-HOOK)
├── schemas/primitives/protocol.ts          ProtocolSchema + Protocols map (LUCX-HOOK)
└── pages/inbounds/form/protocols/index.ts  AwgFields export (LUCX-HOOK)

bin/install-awg-module.sh          DKMS build of amneziawg kernel module + tools
install.sh                         Calls bin/install-awg-module.sh (LUCX-HOOK)
```

---

## Test Commands

```bash
# Go unit tests (no cgo required)
go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v

# Frontend
cd frontend && npm run typecheck && npm run lint

# Full project build (requires frontend/dist)
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .
```

---

## Deploy

- **Target:** `vps_finland-lucx` (SSH alias in `~/.ssh/config`)
- **Service:** `x-ui.service` (systemd)
- **Procedure:** SCP binary → `sudo systemctl restart x-ui` → verify `systemctl status x-ui` + logs
- **AWG runtime check:** `awg show` should list active interfaces; `ip link show awgN` for TUN

---

## Commit Convention

- Префиксы: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`
- Область: `feat(awg): ...`, `fix(frontend): ...`, `chore(codegen): ...`
- Сообщения коммитов — на русском (если не запрошено иное)
- Пример: `feat(awg): порт изолированных пакетов на v3.5.0`

---

## Known Issues

### 1. ~~AWG sidecar раздут относительно mtproto (эталона)~~ — ЗАКРЫТО

**Решено (2026-07-13):** рефактор удалением мёртвого кода. Файлы `params.go`, `cps.go`, `config.go`, `templates.go`, `types.go`, `helpers.go` + 5 тестов были полностью мёртвым кодом — их функции (`GenerateAWGParams`, `GenerateCPS`, `BuildServerConfig`, `RenderPostUp` и др.) вызывались только тестами, ни один живой call site их не использовал. Генерация ключей/обфускации делается во frontend (`createDefaultAwgInboundSettings`). AWG сокращён с 19 до 8 файлов (6 .go + 2 теста) — почти симметрично mtproto (9 файлов). Обновления upstream теперь требуют переноса ~20 файлов вместо 29.

### 2. Сайдкар не проверен в реальном runtime на VPS

Unit-тесты AWG проходят (`go test ./internal/awg/... → ok`). Но реальный запуск kernel-интерфейса `awg-quick` + Xray TUN-inbound на `vps_finland_lucx` не подтверждён в этой сессии. Проверка отложена до завершения миграции на v3.5.0.

### 3. Dependabot отключён (временно)

10 dependabot PR были закрыты при очистке перед миграцией на v3.5.0. После стабилизации миграции — решить: включить обратно или управлять зависимостями вручную.

---

## Frontend Conventions

- Ant Design 6 only — no Tailwind/shadcn.
- TS strict; `@typescript-eslint/no-explicit-any` is an error. Zod schemas in `src/schemas/` are the source of truth; infer types with `z.infer`, never hand-write. Do not edit `src/generated/`.
- Editing `frontend/src` does NOT change what users see until the Vite build is regenerated into `internal/web/dist/`.
- After touching share-link logic (`src/lib/xray/`), run `npm run test` (golden fixtures).

---

## Go Conventions

- Stdlib `testing` only (no testify). Table-driven, `t.Run` subtests.
- NO `//` line comments in committed Go/TS (except directives like `//go:build`). Names carry meaning. (Inherited from upstream CLAUDE.md — applies to upstream code; LucX HOOK blocks may carry the `// LUCX-HOOK:` marker comment by design.)
- `golangci-lint run` / `make lint` for formatting (gofumpt + goimports).
- Conventional-commit prefixes, Russian commit messages.

---

## Debugging Patterns

### Pattern 1: AWG inbound не стартует
- **Cause:** `awg-quick` не установлен или kernel module не загружен.
- **Fix:** `bin/install-awg-module.sh` на сервере. Проверить `awg show`, `ip link show awgN`.

### Pattern 2: LUCX-HOOK конфликт при upstream sync
- **Cause:** Upstream изменил файл с HOOK-маркером между релизами.
- **Fix:** Сравнить старую и новую версию upstream-файла, вручную перенести HOOK-блок в новую структуру. Не `git checkout` весь файл — потеряешь upstream-изменения.

### Pattern 3: Frontend не видит AWG-протокол
- **Cause:** Забыта регистрация в одном из: `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`.
- **Fix:** `grep -rn "awg\|Awg\|AWG" frontend/src/` — проверить все 5 точек регистрации.