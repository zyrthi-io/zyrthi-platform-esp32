#!/bin/bash
set -e

ESP_DIR="${ESP_DIR:-/tmp/esp}"
IDF_PATH="$ESP_DIR/idf"
IDF_TOOLS_PATH="$ESP_DIR/tools"

# 检查是否需要解压
if [ ! -d "$ESP_DIR/idf" ] || [ ! -d "$ESP_DIR/tools" ]; then
    echo "解压 ESP-IDF 和工具链..."
    mkdir -p "$ESP_DIR"
    
    # 解压 IDF
    if [ -f /opt/esp/idf.tar.zst ]; then
        tar -I zstd -xf /opt/esp/idf.tar.zst -C "$ESP_DIR"
        echo "ESP-IDF 解压完成"
    fi
    
    # 解压工具链
    if [ -f /opt/esp/tools.tar.zst ]; then
        tar -I zstd -xf /opt/esp/tools.tar.zst -C "$ESP_DIR"
        echo "工具链解压完成"
    fi
fi

# 设置环境
export IDF_PATH
export IDF_TOOLS_PATH
export IDF_SKIP_CHECK_SUBMODULES=1

# 添加工具链到 PATH
export PATH="$IDF_TOOLS_PATH/tools/xtensa-esp-elf/esp-13.2.0_20240530/xtensa-esp-elf/bin:$PATH"
export PATH="$IDF_TOOLS_PATH/tools/riscv32-esp-elf/esp-13.2.0_20240530/riscv32-esp-elf/bin:$PATH"
export PATH="$IDF_TOOLS_PATH/tools/esp32ulp-elf/2.38_20240113/esp32ulp-elf/bin:$PATH"
export PATH="$IDF_TOOLS_PATH/python_env/idf5.3_py3.11_env/bin:$PATH"
export PATH="$IDF_PATH/tools:$PATH"

# 执行命令
exec "$@"
