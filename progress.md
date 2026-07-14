# LucX-UI — Прогресс миграции на v3.5.0

> Файл ведётся агентом в ходе работы. Обновляется при каждом шаге.

---

## Контекст

- **Репозиторий:** [AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) — форк 3x-ui
- **Цель:** миграция с v3.3.1 → v3.5.0 (228 коммитов апстрима)
- **Стратегия:** migrate (по AGENTS.md) — свежий checkout `origin/main` v3.5.0 + перенос LucX-кода поверх
- **Ветка миграции:** `feat/awg-sidecar-v3.5.0` (создана от `origin/main` v3.5.0)
- **Старая ветка:** `feat/awg-sidecar` (на v3.3.1, эталон для переноса)
- **Дата начала:** 2026-07-13

---

## План

### Этап 1. Очистка мусора ✅
- [x] Закрыть 10 dependabot PR (#1-#12) на GitHub
- [x] Удалить 10 dependabot/* веток на GitHub
- [x] Удалить старую ветку `feature/awg-integration` (локально + удалённо)
- [x] Удалить старую ветку `lucx-ui-phase1` (локально + удалённо)

### Этап 2. Миграция на v3.5.0
- [x] Создать чистую ветку `feat/awg-sidecar-v3.5.0` от `origin/main` (v3.5.0)
- [x] Перенести 29 изолированных LucX-файлов (internal/awg, internal/lucx, migrate_awg, frontend awg.ts/awg.tsx, bin/install-awg-module.sh, awg_job.go) — закоммичено
- [x] Восстановить LUCX-HOOK маркеры в upstream-файлах v3.5.0:
  - [x] `model.go` — AWG Protocol const + validate oneof
  - [x] `db.go` — вызов `pruneLegacyAwgHiddenChildren`
  - [x] `runtime/local.go` — AWG делегирование в AddInbound/DelInbound/AddUser/RemoveUser
  - [x] `service/xray.go` — AWG exclusion + `injectAwgEgress`
  - [x] `web.go` — import awg + cron wiring + StopAll
  - [x] `install.sh` — вызов `bin/install-awg-module.sh`
  - [x] `xray_config_inject_test.go` — тесты injectAwgEgress (5 тестов)
  - [x] frontend: `inbound-defaults.ts`, `schemas/inbound/index.ts`, `primitives/protocol.ts`, `InboundFormModal.tsx`, `protocols/index.ts`
- [x] Прогнать тесты: go test + frontend typecheck/lint
- [x] Frontend: `npm run build` → `internal/web/dist/` собран
- [x] `go build -o /tmp/x-ui .` → exit 0, бинарник 111 МБ
- [ ] Коммит миграции + обновление progress.md/AGENTS.md

### Этап 3. Деплой и проверка (после миграции)
- [ ] SCP на vps_finland_lucx, рестарт x-ui.service
- [ ] Проверка `systemctl status x-ui`, логи
- [ ] Проверка реального запуска AWG kernel-интерфейса

---

## Что сделано

### 2026-07-13

**Очистка мусора:**
- Закрыты 10 dependabot PR (#1-#12) с ветками на GitHub
- Удалены старые ветки `feature/awg-integration` и `lucx-ui-phase1` (локально + удалённо)
- На GitHub осталось 2 ветки: `feat/awg-sidecar`, `main`. Открытых PR нет.

**Миграция — подготовка:**
- Создана ветка `feat/awg-sidecar-v3.5.0` от `origin/main` (v3.5.0, commit `4e928a1c`)
- Перенесены и закоммичены 29 изолированных LucX-файлов:
  - `internal/awg/` — 19 файлов (manager, process, instance, traffic, orphans, params, cps, config, types, templates, helpers + тесты)
  - `internal/lucx/` — 6 файлов (parser, nodetype, outbound_link + тесты)
  - `internal/database/migrate_awg.go` + тест
  - `internal/web/job/awg_job.go`
  - `frontend/src/schemas/protocols/inbound/awg.ts` + `frontend/src/pages/inbounds/form/protocols/awg.tsx`
  - `bin/install-awg-module.sh`

**Миграция — LUCX-HOOK в upstream-файлах:**
- `model.go`: добавлен `AWG Protocol = "awg"` const + `awg` в validate oneof
- `db.go`: добавлен вызов `pruneLegacyAwgHiddenChildren()` в `initModels()`
- `runtime/local.go`: import `internal/awg` + делегирование AddInbound/DelInbound/AddUser/RemoveUser
- `service/xray.go`: AWG exclusion в цикле генерации конфига + `injectAwgEgress` (TUN inbound + routing rule)
- `web.go`: import `internal/awg` + `cadenceAwg` const + cron-задача `awgJob` + `awg.GetManager().StopAll()` в shutdown
- `install.sh`: вызов `bin/install-awg-module.sh` после `setup_fail2ban`
- `xray_config_inject_test.go`: 5 тестов `injectAwgEgress` (WithOutbound, NoOutbound, Disabled, TagCollision, DefaultMTU)
- `inbound-defaults.ts`: import `AwgInboundSettings` + `createDefaultAwgInboundSettings` + `AnyInboundSettings` + switch case
- `schemas/protocols/inbound/index.ts`: import + export + discriminated union
- `primitives/protocol.ts`: `'awg'` в enum + `AWG` в Protocols map
- `InboundFormModal.tsx`: import `AwgFields` + рендер `protocol === Protocols.AWG`
- `protocols/index.ts`: export `AwgFields`

**Тесты:**
- `go test ./internal/awg/...` → ok 2.306s ✅
- `go test ./internal/lucx/...` → ok (nodetype, outbound_link, parser) ✅
- `go test ./internal/database/model` → ok ✅ (проверяет AWG const)
- `internal/database` (с cgo) — не запущен: gcc отсутствует на Windows. На Linux/VPS сработает.
- `npm run typecheck` → чисто ✅
- `npm run lint` → чисто ✅
- `npm run build` → `internal/web/dist/` собран ✅
- `go build -o /tmp/x-ui .` → exit 0, бинарник 111 МБ ✅

**Документация:**
- Создан актуальный `AGENTS.md` на новой ветке (старый был только на `feat/awg-sidecar`). Изучены AGENTS.md из соседних проектов (angry-box, AwgToolza) + CLAUDE.md из lucx-ui. Добавлены: версия v3.5.0, философия минимального внедрения, Known Issues (раздутость AWG, непроверённый runtime, dependabot), деплой, конвенции коммитов на русском, debugging patterns.
- Ведётся этот файл `progress.md`.

---

## Архитектурное решение (2026-07-13)

**Вопрос:** AWG сайдкар имеет 19 файлов vs 9 у mtproto (эталон). Лишние 7 файлов — генерация конфига/обфускации (params/cps/templates/config/types/helpers/traffic), которой у mtproto нет (mtg — готовый бинарщик).

**Решение пользователя:** оставить как есть, добить миграцию. Рефактор архитектуры отложен.

## Рефактор AWG — удаление мёртвого кода (2026-07-13)

**Исследование подтвердило:** 6 файлов AWG (`params.go`, `cps.go`, `config.go`, `templates.go`, `types.go`, `helpers.go`) + 5 тестов — полностью мёртвый код. Их функции (`GenerateAWGParams`, `GenerateCPS`, `BuildServerConfig`, `BuildClientConfig`, `UpdateServerConfig`, `RenderPostUp`, `RenderPostDown`, `MergeParamsToSettings`, `ValidateAWGParams`, `GenKey`, `GenPSK`, `DerivePubkey`) вызывались ТОЛЬКО тестами. Ни один живой call site (manager/process/instance/traffic/orphans/job/runtime/web/frontend) их не использовал.

Генерация ключей и обфускации делается во frontend (`createDefaultAwgInboundSettings` в `inbound-defaults.ts` — `Wireguard.generateKeypair` + `Math.random`). Go-генераторы были дубликатом. Комментарий во frontend «backend regenerates obfuscation when obfLevel/profile change» — ложный, такой логики в Go нет.

**Выполнено:**
- Перенесено в `process.go`: `awgConfigDir` (const) + `awgQuick` (func) + `"os/exec"` импорт
- Удалено 6 .go: params/cps/config/templates/types/helpers
- Удалено 5 тестов: config_test, config_roundtrip_test, cps_test, params_test, templates_test
- Поправлен комментарий manager.go (упоминание BuildServerConfig)
- Обновлён AGENTS.md (Architecture Map, Known Issue #1 → ЗАКРЫТО)

**Результат:** 19 файлов → 8 файлов (6 .go + 2 теста). Почти симметрично mtproto (9 файлов).

**Проверки:**
- `go build ./internal/awg/...` → exit 0 ✅
- `go test ./internal/awg/...` → ok 0.903s, все 11 тестов PASS ✅
- `go build -o /tmp/x-ui .` → exit 0 ✅
- LUCX-HOOK count → 48 (не изменилось) ✅

## Dependabot — ужесточение (2026-07-13)

**Решение пользователя:** security + урезанный scope.

**Выполнено:** `.github/dependabot.yml` — секция `updates: []` (version updates отключены). Security updates (CVE) остаются через GitHub Settings. Режим: PR только при найденной уязвимости, без еженедельного шума минорных версий npm/gomod/github-actions. Шаблон для возврата version updates оставлен в комментарии в yml-файле.

AGENTS.md Known Issue #3 обновлён.

## Адаптация install.sh + release-процесс (2026-07-13)

**Цель:** `install.sh` должен ставить нашу сборку так же просто, как апстрим — через GitHub-релиз.

**Выполнено:**
- `install.sh` — 8 LUCX-HOOK замен URL (`MHSanaei/3x-ui` → `AlexeyLCP/lucx-ui`):
  - Константы `LUCX_REPO`/`LUCX_BRANCH` вверху
  - api.github.com/releases/latest, release tarball download, fallback URL → наш репо
  - x-ui.sh, x-ui.rc, 3 service-юнита → raw.githubusercontent из нашей ветки
- `bin/build-release.sh` — новый скрипт сборки релиза на VPS:
  - Клон форка → `npm build` → `CGO_ENABLED=1 go build` (с gcc на VPS)
  - Скачивание Xray+mtg из апстрим-релиза `MHSanaei/3x-ui` v3.5.0
  - Упаковка `x-ui-linux-amd64.tar.gz` (структура как у апстрима)
  - Инструкция по созданию GitHub-релиза
- Оба скрипта проходят `bash -n`
- Коммит `8b627f8e` запушен

**Почему сборка на VPS:** CGO-бинарник (mattn/go-sqlite3) нельзя cross-compile с Windows на Linux — нужен gcc + linux-заголовки. Cross-compile `CGO_ENABLED=0` соберётся, но при запуске упадёт (sqlite stub).

**Инструкция для VPS** — в AGENTS.md (секция Release & Install) и в выводе `bin/build-release.sh`.

**Следующие шаги (требует VPS):**
1. На VPS: `curl .../bin/build-release.sh | bash` → `/tmp/x-ui-linux-amd64.tar.gz`
2. `gh release create v3.5.0-lucx.1 /tmp/x-ui-linux-amd64.tar.gz --repo AlexeyLCP/lucx-ui`
3. `bash <(curl .../install.sh)` → установка панели с нашим кодом
4. UI → создать AWG-inbound → `awg show` / `ip link show awg0`

## Фаза 1: Клиентские .conf + share-link + создание клиентов (2026-07-13)

**Проблема:** установка работала, подключение создавалось, но НЕ было генерации клиентских конфигов, share-link, создания пользователей (peers).

**Решение:** портированы паттерны WireGuard (эталон в репо) — клиентский Curve25519 keypair + PSK + туннельный адрес хранятся сервером, полный .conf и amneziawg:// share-link собираются одним кликом.

**Источники:** pumbaX/awg-multi-script (генерация конфигов), hoaxisr/awg-manager (скан хоста — отложен в Фазу 3).

**Backend:**
- `internal/awg/instance.go`: PeerSpec расширен (PrivateKey, AllowedIPs /32, Keepalive); InstanceFromInbound парсит новые поля (publicKey/privateKey/preSharedKey/allowedIPs/keepAlive) + legacy (id/password), enable как *bool (absent=true для старых inbound'ов)
- `internal/web/service/client_awg.go` (новый): defaultAwgClients — wgutil.GenerateWireguardKeypair, GenerateWireguardPSK, allocateWireguardAddress из 10.8.0.0/24 (отличается от WG 10.0.0.0/24 — без коллизий)
- `internal/web/service/client_inbound_apply.go`: case AWG (5 точек: генерация, валидация, newClientId, перенос ключей при edit, raw-map)
- `internal/sub/service.go`: 'awg' в SQL-фильтр GetSubs, case AWG в GetLink, genAwgLink (amneziawg:// с обфускацией Jc/S1-S4/H1-H4/I1-I5 в query params)
- `internal/awg/manager.go`: renderServerConf уже использует peer.AllowedIPs (туннельный /32)

**Frontend:**
- `schemas/protocols/inbound/awg.ts`: clients[] расширено, убран комментарий 'never stored server-side'
- `lib/xray/inbound-link.ts`: genAwgLink/genAwgConfig/genAwgConfigs/genAwgLinks + case 'awg' в genInboundLinks
- `pages/clients/ClientFormModal.tsx`: MULTI_CLIENT_PROTOCOLS += awg, awgIds/showAwg, regenerateAwgKeys, UI-блок (переиспользует wg-поля, Curve25519 base тот же)

**Проверки:** go build ./... exit 0; go test ./internal/awg/... ok; typecheck + lint чисто.
**Коммит:** a258ca57 (запушен).

**Фаза 2 (CPS-генерация I1-I5) и Фаза 3 (скан хоста) — отдельно.**

## Проверка Фазы 1 на VPS 144.31.224.212 (2026-07-13)

**Окружение:** Debian 13, ядро 6.12, amneziawg kernel-модуль загружен (DKMS).

**Развёрнутый бинарник:** собран через WSL (CGO_ENABLED=1, с Фазой 1) → SCP → `/usr/local/x-ui/x-ui` → systemctl restart x-ui.

**Найденная проблема:** `awg-quick up` падал с `resolvconf: command not found` — Debian 13 не имеет resolvconf по умолчанию, а .conf содержит `DNS =`. Решение: `apt-get install openresolv`. После этого awg1 поднялся (порт 15963, MTU 1320, обфускация применена). TODO: добавить openresolv в `bin/install-awg-module.sh` как зависимость.

**Проверка end-to-end:**
- ✅ x-ui работает (active/enabled)
- ✅ amneziawg kernel-модуль загружен
- ✅ awg1 интерфейс поднят (порт 15963)
- ✅ Клиент вставлен в БД (SQL, т.к. CSRF блокирует curl-логин) → Reconcile применил peer в kernel (`awg show` видит publicKey, allowed ips 10.8.0.2/32, keepalive 25)
- ✅ Подписка (порт 2096, /sub/) отдаёт `amneziawg://` ссылку со всеми параметрами: клиентский privateKey (userinfo), server publicKey, address=10.8.0.2/32, dns, mtu, keepalive, presharedkey, обфускация (jc/jmin/jmax/s1-s4/h1-h4)

**Пример сгенерированной ссылки:**
```
amneziawg://OKtt7...%3D@localhost:15963?address=10.8.0.2%2F32&dns=1.1.1.1...&h1=447248&h2=...&jc=10&jmax=247&jmin=80&keepalive=25&mtu=1320&presharedkey=k2Sb...&publickey=dMeIQ...&s1=39&s2=89&s3=78&s4=72#-testuser1
```

**Замечание:** `localhost` в endpoint — `shareAddrStrategy` дефолт `node`, сервер не знает свой внешний IP. Нужно настроить `webDomain`/`shareAddr` или strategy `custom`. Не баг Фазы 1.

**Фаза 1 подтверждена в проде.**

## Фаза 2: CPS-генерация I1-I5 (pumbaX) — 2026-07-13

**Реализовано:**
- `internal/awg/cps/` — порт pumbaX/awg-multi-script:
  - `domains.go` — домен-пулы RU/WORLD (TLS/DNS/SIP/QUIC), SelectDomain
  - `params.go` — GenerateAWGParams (Jc/Jmin/Jmax/S1-S4/H1-H4 с инвариантами AmneziaWG: Jmin<Jmax, |S1+56-S2|>=10, H1-H4 в 4 непересекающихся квадрантах 2^29)
  - `cps.go` — GenerateCPS (TLS Chrome-like ClientHello с GREASE/SNI/groups/key_share/padding, DNS EDNS0, SIP REGISTER, QUIC v1 Initial + second/short packets)
  - `cps_test.go` — 6 тестов (инварианты 200 итераций, все профили/регионы)
- API: `POST /panel/api/inbounds/awg/generateObfuscation` (awg.go)
- Frontend: awg.tsx — кнопка генерации вызывает backend API (вместо Math.random заглушки), loading state

**Проверено в проде:** API вернуло `success:true` с полным набором — jc/jmin/jmax, s1-s4, h1-h4 (4 квадранта), i1-i5 (TLS ClientHello с разными SNI: reddit/cloudflare/google/github/wikipedia). ✅

**Фикс:** `bin/install-awg-module.sh` — `apt-get install openresolv` (awg-quick падал с 'resolvconf: command not found' на Debian 13).

## Фаза 3: Скан хоста (hoaxisr) — 2026-07-13

**Реализовано:**
- `internal/awg/signature/capture.go` — порт hoaxisr/awg-manager:
  - `Capture(domain)` — отправляет QUIC v1 Initial (с TLS ClientHello SNI=domain) на UDP 443, читает ответы, возвращает I1-I5 как CPS строки
  - Чистый Go: net.Dial UDP, crypto/hkdf (HKDF-SHA256, RFC 9001 §5.2), crypto/cipher (AES-128-GCM), header protection (RFC 9001 §5.4), crypto/tls через net.Pipe для ClientHello
- API: `POST /panel/api/inbounds/awg/captureHost` (awg.go)
- Frontend: awg.tsx — поле "сканировать хост" (Input + кнопка), вызывает API, заполняет I1-I5

**Endpoint работает:** возвращает корректную ошибку "host did not reply on QUIC 443" для хостов без ответа.

**⚠️ Known bug: capture не получает ответов от реальных хостов** (google/cloudflare/dns.google — все таймаутят, получают только свой собственный Initial). `buildTLSClientHello` работает корректно (1487 байт реального ClientHello через net.Pipe), но `buildInitialPacket` (QUIC Initial шифрование: HKDF/AES-GCM/header protection) генерирует невалидный пакет, который серверы отбрасывают. Требует отладки crypto-деталей (возможно: varint кодировка length, nonce XOR, header protection mask позиции, или ClientHello record-wrapping для CRYPTO frame).

**Статус Фазы 3:** endpoint + frontend готовы, но capture пока нерабочий. Фаза 2 (случайная CPS-генерация) полностью покрывает практическую потребность — скан хоста опциональная фича.

## Релиз v3.5.0-lucx.3 + фикс инверсии onlyI1 (2026-07-14)

**Баг:** `GenerateCPS` 4-й параметр `onlyI1` (true=только I1), а API поле `FullI1I5` (true=все I1-I5). В awg.go передавалось `req.FullI1I5` напрямую — инверсия: при `fullI1I5=true` получали только I1, при `false` — все 5. Исправлено: `!req.FullI1I5`.

**Релизы через CI:**
- `v3.5.0-lucx.1` (устарел, удалён) — Фазы 1
- `v3.5.0-lucx.2` (устарел, удалён) — Фазы 1-3 (с багом onlyI1)
- `v3.5.0-lucx.3` (latest) — Фазы 1-3 + фикс onlyI1 ✅

**CI:** `release.yml` упрощён (только linux/amd64), триггер по push тега `v*.*.*` → авто-сборка через Bootlin musl static + загрузка asset. Релиз делается latest вручную после CI.

**Проверка v3.5.0-lucx.3 на VPS 144.31.224.212 (из релиза, не ручной сборки):**
- ✅ `install.sh` качает и ставит v3.5.0-lucx.3
- ✅ awg1 поднят, peer в kernel (порт 15963)
- ✅ generateObfuscation API: QUIC full → I1=2402, I2=762, I3=172, I4=122, I5=114 (все 5 пакетов)
- ✅ jc/jmin/jmax/s1-s4/h1-h4 (4 квадранта) — корректно
- ✅ amneziawg:// подписка работает

**Обновления upstream теперь:** ручной перенос ~20 файлов вместо 29.

---

## Заметки

- v3.5.0 релиз 2026-07-12 (вчера)
- 228 коммитов между v3.3.1 и v3.5.0
- 41 LUCX-HOOK маркер на старой ветке
- Все 8 upstream-файлов с HOOK-маркерами изменились между v3.3.1 и v3.5.0 — требуется ручное восстановление
- Тесты AWG на старой ветке проходят: `go test ./internal/awg/... → ok 2.212s`