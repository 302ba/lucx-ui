#!/bin/bash
# Copyright (c) 2025 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# LucX-UI Component. Free for personal and educational use.
# Commercial use (including VPN resale) requires explicit written permission from the author.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

set -e

# =============================================================================
# LucX-UI: Установка модуля ядра AmneziaWG (DKMS + update-initramfs)
#
# Универсальный скрипт — работает на Debian/Ubuntu/Armbian с любым ядром.
# Обходит проблему "linux-headers-$(uname -r) не найден" через fallback на
# meta-package + предложение reboot если ядро обновилось но не загружено.
# Подход перенят из pumbaX/awg-multi-script (do_install).
# =============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'

echo -e "${GREEN}=== Установка модуля ядра AmneziaWG ===${NC}"

[[ $EUID -ne 0 ]] && { echo -e "${RED}Запустите с правами root${NC}"; exit 1; }

# Check if already loaded
if [[ -d /sys/module/amneziawg ]]; then
    echo -e "${GREEN}Модуль amneziawg уже загружен.${NC}"
    command -v awg &>/dev/null && { echo -e "${GREEN}awg уже установлен.${NC}"; exit 0; }
fi

# Detect OS
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    OS_ID=$ID
else
    echo -e "${RED}Не удалось определить ОС (/etc/os-release отсутствует)${NC}"
    exit 1
fi

# 1. Install build dependencies
echo -e "${GREEN}Установка сборочных зависимостей...${NC}"
apt-get update -qq

# Core build tools + DKMS + git
apt-get install -y -q build-essential dkms git unzip curl 2>/dev/null || true

# openresolv — awg-quick вызывает resolvconf при наличии DNS= в .conf.
# Без него awg-quick up падает с "resolvconf: command not found".
apt-get install -y -q openresolv 2>/dev/null || echo -e "${YELLOW}openresolv не установлен — awg-quick может падать на DNS=${NC}"

# 2. Install kernel headers — универсальная логика с fallback
RUNNING_KERNEL=$(uname -r)
echo -e "${GREEN}Ядро: ${RUNNING_KERNEL}${NC}"

# Сначала пробуем точный пакет headers для текущего ядра
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    echo -e "${GREEN}Установка linux-headers для ${RUNNING_KERNEL}...${NC}"
    apt-get install -y -q "linux-headers-${RUNNING_KERNEL}" 2>/dev/null || true
fi

# Если точный пакет не найден — fallback на meta-package
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    echo -e "${YELLOW}Точные headers не найдены, пробуем meta-package...${NC}"
    case "$OS_ID" in
        ubuntu|debian|linuxmint|raspbian)
            apt-get install -y -q linux-headers-amd64 2>/dev/null || \
            apt-get install -y -q linux-headers-generic 2>/dev/null || \
            apt-get install -y -q linux-headers-generic-hwe-22.04 2>/dev/null || true
            ;;
        armbian)
            apt-get install -y -q linux-headers-current-sunxi 2>/dev/null || \
            apt-get install -y -q linux-headers-current-rockchip 2>/dev/null || \
            apt-get install -y -q linux-headers-current-arm64 2>/dev/null || true
            ;;
        *)
            apt-get install -y -q linux-headers-amd64 2>/dev/null || \
            apt-get install -y -q linux-headers-generic 2>/dev/null || true
            ;;
    esac
fi

# Если headers всё ещё нет — возможно ядро обновилось но не загружено
if [[ ! -d "/lib/modules/${RUNNING_KERNEL}/build" ]]; then
    # Проверим — есть ли headers для НОВОГО ядра (не загруженного)
    NEWEST_HEADERS=$(ls -d /lib/modules/*/build 2>/dev/null | head -1)
    if [[ -n "$NEWEST_HEADERS" ]]; then
        NEWEST_KERNEL=$(basename $(dirname "$NEWEST_HEADERS"))
        echo -e "${YELLOW}┌──────────────────────────────────────────────────────────┐${NC}"
        echo -e "${YELLOW}│ Headers найдены для ${NEWEST_KERNEL}, но загружено ${RUNNING_KERNEL}        │${NC}"
        echo -e "${YELLOW}│ Ядро обновилось но не загружено. Нужен REBOOT.           │${NC}"
        echo -e "${YELLOW}│ После reboot запустите этот скрипт снова.                │${NC}"
        echo -e "${YELLOW}└──────────────────────────────────────────────────────────┘${NC}"
        echo -e "${GREEN}Выполняю reboot...${NC}"
        sleep 3
        reboot
        exit 0
    fi
    echo -e "${RED}Заголовки ядра для ${RUNNING_KERNEL} не найдены.${NC}"
    echo -e "${YELLOW}Попробуй: apt-get install linux-headers-${RUNNING_KERNEL}${NC}"
    echo -e "${YELLOW}Или обнови ядро: apt-get install linux-image-amd64 && reboot${NC}"
    exit 1
fi
echo -e "${GREEN}Заголовки ядра: OK${NC}"

# 3. Build and install kernel module via DKMS
if [[ ! -d /sys/module/amneziawg ]]; then
    echo -e "${GREEN}Сборка модуля ядра из исходников...${NC}"
    KERNEL_MOD_DIR="/tmp/amneziawg-kmod-$$"
    rm -rf "$KERNEL_MOD_DIR"
    git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git "$KERNEL_MOD_DIR"
    cd "$KERNEL_MOD_DIR/src"

    make dkms-install 2>/dev/null || true
    MOD_VER=$(grep -oP 'version\s*"\K[^"]+' dkms.conf 2>/dev/null || echo "1.0.0")
    dkms add -m amneziawg -v "$MOD_VER" 2>/dev/null || true
    dkms build -m amneziawg -v "$MOD_VER" || {
        echo -e "${RED}Ошибка сборки DKMS. Проверь заголовки ядра.${NC}"
        exit 1
    }
    dkms install -m amneziawg -v "$MOD_VER"

    cd /tmp; rm -rf "$KERNEL_MOD_DIR"
    echo -e "${GREEN}Модуль ядра собран и установлен.${NC}"
fi

# 4. Build and install userspace tools
if ! command -v awg &>/dev/null; then
    echo -e "${GREEN}Сборка утилит awg...${NC}"
    TOOLS_DIR="/tmp/amneziawg-tools-$$"
    rm -rf "$TOOLS_DIR"
    git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-tools.git "$TOOLS_DIR"
    cd "$TOOLS_DIR/src"
    make && make install
    cd /tmp; rm -rf "$TOOLS_DIR"
    echo -e "${GREEN}Утилиты awg установлены.${NC}"
fi

# 5. Load module and enable autostart
modprobe amneziawg 2>/dev/null || {
    echo -e "${YELLOW}Не удалось загрузить модоль. Возможно, нужен ребут.${NC}"
}
echo "amneziawg" > /etc/modules-load.d/amneziawg.conf

# 6. Update initramfs (critical for reboot survival)
echo -e "${GREEN}Обновление initramfs...${NC}"
update-initramfs -u -k all 2>/dev/null || update-initramfs -u 2>/dev/null || {
    echo -e "${YELLOW}Предупреждение: update-initramfs не сработал. Модуль может не загрузиться после ребута.${NC}"
}

# 7. Secure Boot check
if [[ -d /sys/firmware/efi ]]; then
    if mokutil --sb-state 2>/dev/null | grep -q "SecureBoot enabled"; then
        echo -e "${YELLOW}┌──────────────────────────────────────────────────────┐${NC}"
        echo -e "${YELLOW}│ ОБНАРУЖЕН SECURE BOOT!                              │${NC}"
        echo -e "${YELLOW}│ Модуль amneziawg не подписан — может не загрузиться. │${NC}"
        echo -e "${YELLOW}│ Отключи Secure Boot в BIOS или подпиши модуль.       │${NC}"
        echo -e "${YELLOW}└──────────────────────────────────────────────────────┘${NC}"
    fi
fi

# 8. Verify
echo ""
if lsmod | grep -q amneziawg; then
    echo -e "${GREEN}✓ Модуль amneziawg загружен${NC}"
else
    echo -e "${YELLOW}⚠ Модуль не загружен — нужен ребут${NC}"
fi
command -v awg &>/dev/null && echo -e "${GREEN}✓ awg установлен ($(awg version 2>&1 | head -1))${NC}"
command -v awg-quick &>/dev/null && echo -e "${GREEN}✓ awg-quick установлен${NC}"
command -v resolvconf &>/dev/null && echo -e "${GREEN}✓ resolvconf (openresolv) установлен${NC}"
echo -e "${GREEN}=== Установка AWG завершена ===${NC}"
