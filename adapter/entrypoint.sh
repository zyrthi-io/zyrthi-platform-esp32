#!/bin/bash
# zyrthi ESP32 容器入口点

set -e

COMMAND=${1:-help}
shift || true

case "$COMMAND" in
    build)
        exec python3 /opt/zyrthi/adapter/build.py "$@"
        ;;
    flash)
        exec python3 /opt/zyrthi/adapter/flash.py "$@"
        ;;
    monitor)
        exec python3 /opt/zyrthi/adapter/monitor.py "$@"
        ;;
    help|--help|-h)
        echo "zyrthi ESP32 Platform Container"
        echo ""
        echo "Usage: <image> <command> [options]"
        echo ""
        echo "Commands:"
        echo "  build    - Build firmware"
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
