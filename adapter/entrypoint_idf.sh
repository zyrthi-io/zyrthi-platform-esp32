#!/bin/bash
# zyrthi ESP32 容器入口点 (基于 ESP-IDF)

# 创建空的 submodule 目录（绕过 Gitee 镜像的 submodule 认证问题）
mkdir -p $IDF_PATH/components/bt/controller/lib_esp32/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bt/controller/lib_esp32c3/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bt/controller/lib_esp32c6/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bt/controller/lib_esp32h2/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bt/controller/lib_esp32s3/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bt/esp_ble_mesh/lib/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bt/host/nimble/nimble/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/esp_wifi/lib/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/esp_phy/lib/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/esp_coex/lib/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/openthread/lib/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/mbedtls/mbedtls/.git 2>/dev/null || true
mkdir -p $IDF_PATH/components/bootloader/subproject/components/micro-ecc/micro-ecc/.git 2>/dev/null || true

# 设置 ESP-IDF 环境
source $IDF_PATH/export.sh 2>/dev/null || true

set -e

COMMAND=${1:-help}
shift || true

case "$COMMAND" in
    install)
        # ESP-IDF 工具链已在镜像构建时安装
        echo '{"success": true, "message": "ESP-IDF 工具链已就绪"}'
        ;;
    build)
        # 支持命令行参数: build <chip> [sources...]
        if [ -n "$1" ]; then
            CHIP="$1"
            shift
            # 构造 JSON 输入
            JSON="{\"chip\": \"${CHIP}\""
            if [ -n "$*" ]; then
                # 将源文件参数转为 JSON 数组
                SOURCES=$(echo "$*" | sed 's/ /", "/g')
                JSON="${JSON}, \"sources\": [\"${SOURCES}\"]}"
            else
                JSON="${JSON}}"
            fi
            echo "$JSON" | exec python3 /opt/zyrthi/adapter/build_idf.py
        else
            # 从 stdin 读取 JSON
            exec python3 /opt/zyrthi/adapter/build_idf.py
        fi
        ;;
    flash)
        exec python3 /opt/zyrthi/adapter/flash.py "$@"
        ;;
    monitor)
        exec python3 /opt/zyrthi/adapter/monitor.py "$@"
        ;;
    help|--help|-h)
        echo "zyrthi ESP32 Platform Container (ESP-IDF v5.3)"
        echo ""
        echo "Usage: <image> <command> [options]"
        echo ""
        echo "Commands:"
        echo "  install  - Check toolchain (already installed)"
        echo "  build    - Build firmware (generates CMakeLists.txt)"
        echo "  flash    - Flash firmware to device"
        echo "  monitor  - Serial monitor"
        echo ""
        echo "Input/Output: JSON via stdin/stdout"
        exit 0
        ;;
    *)
        echo "Unknown command: $COMMAND" >&2
        exit 1
        ;;
esac
