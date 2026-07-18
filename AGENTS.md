# LucX-UI — Agent Operating Manual

This file is the law for every agent working on this project. Read it completely before touching any code.

---

## Project Overview

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (currently **v3.5.0**) that adds native AmneziaWG (AWG) support as a kernel-interface sidecar, mirroring upstream's MTProto (mtg) sidecar architecture. LucX-specific code lives in `internal/awg/` and `internal/lucx/`; all integration points in upstream files are wrapped in `LUCX-HOOK` / `END LUCX-HOOK` markers.

**Upstream sync strategy:** migrate (not rebase). Each upstream release = fresh checkout of `origin/main` + port LucX code on top via LUCX-HOOK markers. The old `.patch`-file system is gone; integration is inline.

**Remotes:**
- `origin` → `MHSanaei/3x-ui` (upstream)
- `gh` → `AlexeyLCP/lucx-ui` (our fork)

**Active branch:** `main` (слит с `main`, миграция v3.5.0 завершена).

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
4. BRANCH  → Work on `main` (активная ветка, миграция v3.5.0 слита)
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
11.5. CHECK PR/ISSUES → ПЕРЕД пушем ВСЕГДА проверяй открытые PR и issues:
             `gh pr list --repo AlexeyLCP/lucx-ui --state open`
             `gh issue list --repo AlexeyLCP/lucx-ui --state open`
             Если есть необработанные PR (не от тебя) или issues — НЕ пушь
             сразу. Сообщи пользователю: какие PR/issues открыты, кем, и
             предложи: (а) сначала проверить/смержить PR, (б) сначала
             исправить issue, (в) пушить после. Не пушь молча поверх
             чужого PR — можно затереть или сломать чужую работу.
12. DOCS   → ВСЕГДА актуализируй progress.md и AGENTS.md. Каждый коммит — новая
             запись в progress.md (что сделано, какой lucxVersion, какие файлы,
             какие тесты). При изменении архитектуры — обнови AGENTS.md
             (Architecture Map, Known Issues, Debug Patterns). НЕ оставляй
             пробелов: если сделал фикс — запиши его. Файлы — закон проекта.
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

- **Manager** (`internal/awg/manager.go`): singleton with `Ensure`/`Reconcile`/`StopAll`/`CollectTraffic`/`SyncPeers`, fingerprint-based restart on config change, orphan sweep at first call. Reconcile-loop convergence: `ensureXrayRouting` (routeThroughXray: table/rule into tunN, dies with tunN on Xray restart) + `ensureNatRules` (kernel NAT: MASQUERADE/FORWARD, dies on iptables flush — fail2ban/docker).
- **Process** (`internal/awg/process.go`): wraps `awg-quick up/down` (kernel interface lifecycle, not a daemon). No tun2socks — routing is via Xray TUN inbound.
- **Instance** (`internal/awg/instance.go`): desired runtime state + `InstanceFromInbound` + `fingerprint`.
- **Traffic** (`internal/awg/traffic.go`): `awg show <iface> transfer` parsing for per-peer byte accounting (replaces mtg's Prometheus HTTP scrape).
- **Diagnostics** (`internal/awg/diagnostics.go`): read-only probe chain (interface UP, ip_forward, peers/handshakes, then mode-specific: MASQUERADE+FORWARD or tunN+rule+table). `Diagnose(inst)` → ordered `DiagCheck`s with evidence details; served by `GET /panel/api/inbounds/:id/awgDiagnostics` and rendered by the AWG form's diagnostics modal. Fixes belong to reconcile — diagnostics only makes failures visible.
- **Orphans** (`internal/awg/orphans_{linux,other}.go`): sweep orphaned awg interfaces from a previous x-ui run.
- **Job** (`internal/web/job/awg_job.go`): cron `@every 10s` — Reconcile desired inbounds + fold traffic deltas.
- **Egress** (`internal/web/service/xray.go:injectAwgEgress`): inject TUN inbound into generated Xray config when `routeThroughXray` is set, symmetric with `injectMtprotoEgress`. Per-inbound gateway `10.254.(N%254).1/30` (separate /30 subnet, never conflicts with AWG tunnel subnet). Sniffing `{http,tls,quic, routeOnly:true}` on TUN inbound so domain/geosite rules work for AWG traffic.
- **Runtime** (`internal/web/runtime/local.go`): delegate AWG `AddInbound`/`DelInbound` to `awg.GetManager()`; `AddUser`/`RemoveUser` are no-ops (peer sync via Reconcile).
- **CPS** (`internal/awg/cps/`): CPS packet generators (TLS/DNS/SIP/QUIC) + AWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4). TLS and QUIC have browser-specific fingerprints (Chrome/Firefox/Safari).
- **Signature** (`internal/awg/signature/`): QUIC host capture — sends QUIC Initial to UDP 443, reads replies → I1-I5.
- **Controller** (`internal/web/controller/awg.go`): `generateObfuscation` + `captureHost` + `awgDiagnostics` API endpoints.
- **NAT** (`internal/awg/nat_{linux,other}.go`): `defaultRouteInterface()` for MASQUERADE target.
- **Inbound needRestart** (`internal/web/service/inbound.go`): `awgRoutesThroughXray` — needRestart on AddInbound/DelInbound/UpdateInbound/SetInboundEnable so Xray regenerates config when routeThroughXray toggles.

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
- `frontend/src/schemas/protocols/inbound/awg.ts` — Zod schema (`AwgInboundSettingsSchema`), includes `mimicryProfile`, `browserProfile`, `outboundTag`, `routeThroughXray`
- `frontend/src/pages/inbounds/form/protocols/awg.tsx` — AntD form (`AwgFields`), uses `useFormContext` (react-hook-form), `FormField` (not `Form.Item`), `message.useMessage()` (not `App.useApp()`)
- `frontend/src/lib/xray/inbound-defaults.ts` — `createDefaultAwgInboundSettings` (LUCX-HOOK)
- `frontend/src/lib/xray/inbound-link.ts` — `genAwgLink`/`genAwgConfig` (share-link + .conf generation, I1-I5 written as-is, no double CPS tag wrapping)
- `frontend/src/pages/clients/wireguardConfig.ts` — `buildAwgClientConfig` (full client .conf with obfuscation block)
- `frontend/src/pages/clients/ClientQrModal.tsx` — AWG panel with QR + download
- Registered in `protocols/index.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`

### 10. License

LucX-UI components (`internal/awg/`, `internal/lucx/`, `internal/database/migrate_awg.go`, `internal/web/controller/awg.go`, `internal/web/job/awg_job.go`, `internal/web/service/client_awg.go`, `frontend/src/schemas/protocols/inbound/awg.ts`, `frontend/src/pages/inbounds/form/protocols/awg.tsx`, `frontend/src/pages/inbounds/form/awg-inbound-id-context.ts`, `frontend/src/pages/clients/wireguardConfig.ts`, `bin/install-awg-module.sh`, `bin/check-lucx.sh`, `bin/pre-push`) are licensed under **PolyForm Noncommercial 1.0.0**. Free for personal and educational use. Commercial use (including VPN resale) requires explicit written permission from the author.

Original 3x-ui code remains under GPL-3.0.

**Every new LucX-owned file MUST carry the SPDX header** (see any existing file in `internal/awg/` for the exact 5-line block). Files with `//go:build` tags put the header after the constraint line; shell scripts after the shebang. The full split (which files are PolyForm vs GPL, why, commercial contact) is documented in [LICENSING.md](LICENSING.md); the canonical license text is [LICENSE-PolyForm-Noncommercial.txt](LICENSE-PolyForm-Noncommercial.txt). Upstream files with LUCX-HOOK blocks stay GPL — never put SPDX headers in them.

---

## Architecture Map

```
internal/awg/                      AWG sidecar (mirrors internal/mtproto/)
├── manager.go                     Manager singleton: Ensure/Reconcile/StopAll/CollectTraffic/SyncPeers + renderServerConf/writeServerConfigFile + natPostUpPostDown + ensureXrayRouting (reconcile-loop route maintenance) + ensureNatRules/natRulesFor (reconcile-loop NAT recovery)
├── process.go                     Process wrapping awg-quick up/down + procLogWriter + awgConfigDir + awgQuick
├── instance.go                    Instance + InstanceFromInbound + fingerprint + PeerSpec
├── traffic.go                     scrapeTransfer via `awg show transfer` + Traffic type
├── diagnostics.go                 Diagnose(inst) — read-only probe chain (interface/ip_forward/peers/NAT or TUN rules), prober interface, DiagCheck/Diagnostics
├── orphans_linux.go               killStrayAwgInterfaces
├── orphans_other.go               no-op off Linux
├── nat_linux.go                   defaultRouteInterface() — ip route show default
├── nat_other.go                   no-op off Linux
├── instance_test.go               Instance/fingerprint/render/NAT tests
├── manager_test.go                Manager state-machine tests
└── diagnostics_test.go            diagnose() with fake prober + parsers (route iface, handshakes)

internal/awg/cps/                  CPS packet generators (TLS/DNS/SIP/QUIC) + AWGParams
├── cps.go                         GenerateCPS + tlsPacket (Chrome/Firefox/Safari) + buildChromeHello/buildFirefoxHello/buildSafariHello + DNS/SIP/QUIC packet builders (quicInitialPacket respects browserProfile)
├── domains.go                     MimicryProfile + BrowserProfile + ObfProfile types + domain pools (RU/World)
├── params.go                      GenerateAWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4) + SetRand for tests + rng
└── cps_test.go                    CPS unit tests (all browsers, invariants, signatures, QUIC browser)

internal/awg/signature/            QUIC host capture (hoaxisr port)
├── capture.go                     Capture(domain) — sends QUIC Initial, reads replies → I1-I5
└── capture_test.go                normalizeDomain/fillPackets/varint/HKDF/ClientHello+Initial structure tests

internal/lucx/                     Smart Cluster
├── parser/                        SSH output → NodeCreds
├── nodetype/                      LucX vs vanilla detection (MTProtoVersion)
└── outbound_link/                 Inbound → outbound config generator

internal/database/
├── migrate_awg.go                 pruneLegacyAwgHiddenChildren + stripHiddenKeys
└── migrate_awg_test.go            stripHiddenKeys unit tests

internal/web/
├── runtime/local.go               AWG delegation in AddInbound/DelInbound (LUCX-HOOK)
├── job/awg_job.go                 AwgJob cron — Reconcile + CollectTraffic + ensureXrayRouting + ensureNatRules
├── service/xray.go                injectAwgEgress (TUN inbound + per-inbound gateway + sniffing) + AWG exclusion (LUCX-HOOK)
├── service/inbound.go             awgRoutesThroughXray + needRestart (LUCX-HOOK) + inboundAwgHints
├── service/client_awg.go          defaultAwgClients — keypair + PSK + address allocation
├── service/xray_config_inject_test.go  injectAwgEgress tests (gateway, sniffing, outboundTag)
├── controller/awg.go               generateObfuscation + captureHost + awgDiagnostics API endpoints (LUCX-HOOK)
└── web.go                         cadenceAwg + StopAll wiring (LUCX-HOOK)

internal/database/model/model.go   AWG Protocol const + validate oneof (LUCX-HOOK)
internal/database/db.go            pruneLegacyAwgHiddenChildren call (LUCX-HOOK)

frontend/src/
├── schemas/protocols/inbound/awg.ts        AwgInboundSettingsSchema (Zod)
├── pages/inbounds/form/protocols/awg.tsx   AwgFields (React + AntD) + diagnostics modal
├── pages/inbounds/form/awg-inbound-id-context.ts  editing inbound id provider for diagnostics (LUCX)
├── pages/inbounds/form/InboundFormModal.tsx       AwgInboundIdProvider wrap (LUCX-HOOK)
├── lib/xray/inbound-defaults.ts            createDefaultAwgInboundSettings (LUCX-HOOK)
├── schemas/protocols/inbound/index.ts      InboundSettingsSchema union (LUCX-HOOK)
├── schemas/primitives/protocol.ts          ProtocolSchema + Protocols map (LUCX-HOOK)
└── pages/inbounds/form/protocols/index.ts  AwgFields export (LUCX-HOOK)

bin/install-awg-module.sh          DKMS build of amneziawg kernel module + tools
bin/check-lucx.sh                  gofumpt check for LucX files (37) — run before push; -w autofixes
bin/pre-push                       git hook: check-lucx + fast go tests + PR/issues guard (AGENTS.md 11.5)
install.sh                         Calls bin/install-awg-module.sh (LUCX-HOOK)
LICENSING.md                       GPL-3.0 / PolyForm-NC split documentation
LICENSE-PolyForm-Noncommercial.txt Canonical PolyForm NC 1.0.0 text
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

# Pre-push hygiene (gofumpt on all LucX files — catches Windows/Linux drift before CI)
bin/check-lucx.sh          # check;  bin/check-lucx.sh -w  # autofix

# Optional: install the git hook that runs check-lucx + fast tests + PR/issues guard (step 11.5)
cp bin/pre-push .git/hooks/pre-push && chmod +x .git/hooks/pre-push
```

---

## Deploy

- **Target:** `lucx` (SSH alias in `~/.ssh/config`, GCP Finland)
- **Service:** `x-ui.service` (systemd)
- **Procedure:** SCP binary → `sudo systemctl restart x-ui` → verify `systemctl status x-ui` + logs
- **AWG runtime check:** `awg show` should list active interfaces; `ip link show awgN` for TUN
- **Testers:** VladufQa (ruvds-rdu8b), Kirill Rudenko (runode) — обновляются через `x-ui update` или reinstall

---

## Release & Install (форк)

`install.sh` адаптирован под наш форк (`AlexeyLCP/lucx-ui`): скачивает релиз-tarball и raw-скрипты (x-ui.sh, x-ui.rc, service-юниты) из `main`. Xray-core + mtg переиспользуются из апстрим-релиза `MHSanaei/3x-ui`.

### Сборка релиза (на VPS, Linux/amd64, с gcc + go + node)

CGO-бинарник (mattn/go-sqlite3) нельзя cross-compile с Windows — сборка только на Linux.

```bash
# 1. Собрать tarball
curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/bin/build-release.sh | bash
# → /tmp/x-ui-linux-amd64.tar.gz

# 2. Создать GitHub-релиз (нужен gh CLI с auth)
gh release create v3.5.0-lucx.1 /tmp/x-ui-linux-amd64.tar.gz \
  --repo AlexeyLCP/lucx-ui \
  --title "v3.5.0-lucx.1" \
  --notes "LucX-UI v3.5.0 с AWG-сайдкаром"

# 3. Установить панель (на этом или другом VPS)
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
# → скачает наш релиз, поставит x-ui + systemd + Xray + mtg + fail2ban + AWG-модуль
```

### Зависимости VPS для сборки
- Go 1.23+ (рекомендуется 1.26)
- Node.js 20+ и npm
- gcc (для CGO)
- git, curl, tar

### Структура релиза (как у апстрима)
```
x-ui-linux-amd64.tar.gz → x-ui/
  ├── x-ui                    ← наш бинарник (CGO, собран из форка)
  ├── x-ui.sh, x-ui.rc        ← из репо
  ├── x-ui.service.{debian,arch,rhel}  ← из репо
  └── bin/
      ├── xray-linux-amd64    ← из апстрим-релиза (не наш код)
      ├── mtg-linux-amd64     ← из апстрим-релиза (не наш код)
      └── install-awg-module.sh  ← наш DKMS-скрипт
```

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

### 2. ~~Сайдкар не проверен в реальном runtime на VPS~~ — ЗАКРЫТО

**Решено (2026-07-16):** сайдкар проверен в реальном runtime на VPS тестеров (VladufQa на ruvds-rdu8b, Kirill Rudenko на runode). Kernel routing (без routeThroughXray) работает — handshake, ICMP, HTTPS, traffic. routeThroughXray работает после PR #13 (needRestart + policy routing + sniffing). Релизы v3.5.0-lucx.20–31 протестированы тестерами.

### 3. Dependabot — только security updates

Version updates (еженедельные PR на новые версии) отключены — `updates: []` в `.github/dependabot.yml`. Это убирает шум минорных обновлений npm/gomod/github-actions, которые накапливались как незакрытые PR (10 шт. были закрыты перед миграцией на v3.5.0). Security updates (CVE) остаются включёнными через GitHub Settings → Dependabot security updates — Dependabot автоматически создаст PR при найденной уязвимости в любой зависимости. Чтобы вернуть version updates — замените `updates: []` на полный список (шаблон в комментарии в yml-файле).

### 4. routeThroughXray — сложнее чем mtproto

AWG routeThroughXray **принципиально сложнее** mtproto из-за kernel→userspace моста:

| | mtproto | AWG |
|---|---|---|
| Тип sidecar | userspace daemon (mtg) | kernel module (awg-quick) |
| Тип трафика | TCP (FakeTLS → MTProto) | IP-пакеты (kernel) |
| Мост в Xray | SOCKS5 loopback (TCP) | TUN inbound (IP) |
| Как трафик попадает в Xray | mtg сам dial 127.0.0.1:port | policy routing: `ip rule iif awgN lookup 1000+N` → `default dev tunN` |
| NAT | не нужен (mtg → SOCKS → Xray) | не нужен (Xray → outbound сам натит) |
| needRestart | `mtprotoRoutesThroughXray` в AddInbound/DelInbound/UpdateInbound | `awgRoutesThroughXray` — те же точки (добавлено в PR #13) |
| Route maintenance | не нужен (SOCKS порт постоянный) | `ensureXrayRouting` в reconcile-цикле (10с) — tunN пересоздаётся при каждом рестарте Xray. В kernel-режиме — `ensureNatRules` (тот же цикл): MASQUERADE/FORWARD умирают при iptables flush |
| Sniffing | SOCKS inbound сам делает | TUN inbound — нужен явный `sniffing: {routeOnly:true}` (без него domain rules не работают) |

Not to re-add: tun2socks (заменено TUN inbound), DNS в серверный .conf (ломает системный DNS), фиксированные table 100 + gateway 10.254.254.1/30 (ломают мульти-инбаунд).

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

### Pattern 4: routeThroughXray — нет интернета
- **Cause 1:** needRestart не сработал → Xray не перегенерировал конфиг → TUN не создан.
  **Fix:** Проверить `awgRoutesThroughXray` в `inbound.go` (AddInbound/DelInbound/UpdateInbound/SetInboundEnable).
- **Cause 2:** `ip rule iif awgN lookup 1000+N` отсутствует или маршрут в table 1000+N потерян (tunN пересоздан).
  **Fix:** `ip rule show | grep awg`, `ip route show table 1000+N`. Reconcile-цикл (10с) должен восстановить.
- **Cause 3:** TUN gateway конфликтует с AWG subnet.
  **Fix:** gateway должен быть `10.254.(N%254).1/30` (per-inbound /30, не AWG subnet).
- **Cause 4:** Domain rules не работают (SNI не виден).
  **Fix:** TUN inbound должен иметь `sniffing: {routeOnly:true}`. Проверить `awgEgressTunSniffing` в `xray.go`.

### Pattern 5: Xray падает "this rule has no effective fields"
- **Cause:** Routing rule без `outboundTag`/`balancerTag`/`domain`/`ip` — только `type` и `inboundTag`.
  **Fix:** Проверить routing template config в панели. `injectAwgEgress` не создаёт rule при пустом `outboundTag` (котел Xray). Если rule приходит из template — убрать пустой rule.