#!/bin/bash
set -e

# 创建 /root/.espressif 目录和软链接
mkdir -p /root/.espressif/python_env
ln -sf /opt/tools/espidf.constraints.v5.3.txt /root/.espressif/espidf.constraints.v5.3.txt
ln -sf /opt/tools/python-env/idf5.3_py3.11_env /root/.espressif/python_env/idf5.3_py3.11_env

COMMAND=${1:-help}
shift || true

case "$COMMAND" in
    install) exec python3 /opt/zyrthi/adapter/install.py "$@" ;;
    build)   exec python3 /opt/zyrthi/adapter/build_idf.py "$@" ;;
    flash)   exec python3 /opt/zyrthi/adapter/flash.py "$@" ;;
    monitor) exec python3 /opt/zyrthi/adapter/monitor.py "$@" ;;
    help|--help|-h)
        echo "zyrthi ESP32 Platform Container (ESP-IDF v5.3)"
        echo "Commands: install | build | flash | monitor"
        exit 0 ;;
    *) echo "Unknown command: $COMMAND" >&2; exit 1 ;;
esac
