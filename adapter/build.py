#!/usr/bin/env python3
"""
zyrthi build 适配器
完全复刻 Arduino ESP32 编译流程
支持 ESP-IDF 预编译静态库链接

架构:
  CLI (Go) --stdin JSON--> build.py --gcc--> 固件

路径配置:
  ZYRTHI_HOME 环境变量，默认 ~/.zyrthi/
  工具链: {ZYRTHI_HOME}/tools/toolchain/{arch}/
  SDK: {ZYRTHI_HOME}/tools/sdk/
"""

import json
import os
import subprocess
import sys
import shutil
from pathlib import Path

# 芯片架构映射
CHIP_ARCH = {
    'esp32': 'xtensa',
    'esp32s2': 'xtensa',
    'esp32s3': 'xtensa',
    'esp32c3': 'riscv',
    'esp32c6': 'riscv',
    'esp32h2': 'riscv',
}

# 编译器前缀映射
COMPILER_PREFIX = {
    'xtensa': 'xtensa-esp-elf-',
    'riscv': 'riscv32-esp-elf-',
}

# Arduino 核心库列表（esp32c3）
ARDUINO_CORE_LIBS = [
    # ESP-IDF 组件库
    "-lriscv", "-lesp_driver_gpio", "-lesp_pm", "-lmbedtls", "-lesp_app_format",
    "-lesp_bootloader_format", "-lapp_update", "-lesp_partition", "-lefuse",
    "-lbootloader_support", "-lesp_mm", "-lesp_system", "-lesp_common",
    "-lesp_rom", "-lhal", "-llog", "-lheap", "-lsoc", "-lesp_hw_support", "-lfreertos",
    "-lnewlib", "-lpthread", "-lcxx", "-lesp_timer", "-lesp_driver_gptimer",
    "-lesp_ringbuf", "-lesp_driver_uart", "-lapp_trace", "-lesp_event", "-lnvs_flash",
    "-lesp_driver_spi", "-lesp_driver_i2s", "-lsdmmc", "-lesp_driver_sdspi",
    "-lesp_driver_rmt", "-lesp_driver_tsens", "-lesp_driver_sdm", "-lesp_driver_i2c",
    "-lesp_driver_ledc", "-lesp_driver_usb_serial_jtag", "-ldriver", "-lesp_phy",
    "-lesp_vfs_console", "-lvfs", "-llwip", "-lesp_netif", "-lwpa_supplicant",
    "-lesp_coex", "-lesp_wifi", "-lhttp_parser", "-lesp-tls", "-lesp_adc",
    "-lesp_eth", "-lesp_gdbstub", "-ltcp_transport", "-lesp_http_client",
    "-lesp_http_server", "-lesp_https_ota", "-lespcoredump", "-lmbedtls_2",
    "-lmbedcrypto", "-lmbedx509", "-leverest", "-lp256m", "-lcoexist", "-lcore",
    "-lespnow", "-lmesh", "-lnet80211", "-lpp", "-lsmartconfig", "-lwapi",
    # 工具链库
    "-lgcc", "-lc", "-lm", "-lstdc++",
    # WiFi/BT PHY
    "-lphy", "-lbtbb", "-lesp_phy",
]

# Arduino 链接脚本（按顺序）
ARDUINO_LINK_SCRIPTS = [
    "rom.api.ld",
    "esp32c3.peripherals.ld",
    "esp32c3.rom.ld",
    "esp32c3.rom.api.ld",
    "esp32c3.rom.bt_funcs.ld",
    "esp32c3.rom.libgcc.ld",
    "esp32c3.rom.version.ld",
    "esp32c3.rom.eco3.ld",
    "esp32c3.rom.eco3_bt_funcs.ld",
    "esp32c3.rom.newlib.ld",
    "memory.ld",
    "sections.ld",
]


def get_zyrthi_home() -> Path:
    """获取 zyrthi 安装目录"""
    # 优先使用环境变量
    home = os.environ.get('ZYRTHI_HOME')
    if home:
        return Path(home)
    # 默认 ~/.zyrthi/
    return Path.home() / '.zyrthi'


def get_toolchain_path(arch: str) -> str:
    """获取编译器路径"""
    prefix = COMPILER_PREFIX.get(arch, 'riscv32-esp-elf-')
    gcc_name = f'{prefix}gcc'
    
    # 1. 检查 PATH 中是否有
    gcc_in_path = shutil.which(gcc_name)
    if gcc_in_path:
        return os.path.dirname(gcc_in_path)
    
    # 2. 检查 ZYRTHI_HOME 下的工具链
    zyrthi_home = get_zyrthi_home()
    toolchain_dir = zyrthi_home / 'tools' / 'toolchain' / arch / 'bin'
    gcc = toolchain_dir / gcc_name
    if gcc.exists():
        return str(toolchain_dir)
    
    return ''


def get_sdk_dir(chip: str) -> str:
    """获取 SDK 目录"""
    zyrthi_home = get_zyrthi_home()
    
    # 检查多个可能的位置
    possible_paths = [
        zyrthi_home / 'tools' / 'sdk' / 'esp-idf',
        zyrthi_home / 'tools' / 'sdk' / 'arduino-esp32-libs',
        zyrthi_home / 'platforms' / 'esp32' / 'sdk',
    ]
    
    for sdk_path in possible_paths:
        if sdk_path.exists():
            chip_dir = sdk_path / chip
            if chip_dir.exists():
                return str(chip_dir)
            # 有些 SDK 直接在根目录包含芯片支持
            return str(sdk_path)
    
    return None


def get_esp_idf_includes(sdk_dir: str, chip: str, arch: str) -> list:
    """生成 ESP-IDF include 路径列表（复刻 Arduino）"""
    includes = []
    base = Path(sdk_dir) / 'include'
    
    # sdkconfig.h 路径
    for config_dir in ['dio_qspi', 'qio_qspi']:
        sdkconfig_inc = Path(sdk_dir) / config_dir / 'include'
        if sdkconfig_inc.exists():
            includes.append(str(sdkconfig_inc))
            break
    
    # 核心路径（参考 Arduino pioarduino-build.py）
    core_paths = [
        'newlib/platform_include',
        'log/include',
        'heap/include',
        'esp_common/include',
        'esp_timer/include',
        'soc/include',
        f'soc/{chip}',
        f'soc/{chip}/include',
        'hal/platform_port/include',
        f'hal/{chip}/include',
        'hal/include',
        'esp_rom/include',
        f'esp_rom/include/{chip}',
        f'esp_rom/{chip}',
        'esp_system/include',
        'esp_system/port/soc',
        f'esp_system/port/include/{arch}',
        'esp_system/port/include/private',
        'freertos/config/include',
        'freertos/config/include/freertos',
        f'freertos/config/{arch}/include',
        'freertos/FreeRTOS-Kernel/include',
        f'freertos/FreeRTOS-Kernel/portable/{arch}/include',
        f'freertos/FreeRTOS-Kernel/portable/{arch}/include/freertos',
        'freertos/esp_additions/include',
        'esp_hw_support/include',
        'esp_hw_support/include/soc',
        f'esp_hw_support/include/soc/{chip}',
        'esp_hw_support/dma/include',
        'esp_hw_support/ldo/include',
        f'esp_hw_support/port/{chip}',
        f'esp_hw_support/port/{chip}/include',
        'esp_adc/include',
        'esp_adc/include/esp_adc',
        f'esp_adc/{chip}/include',
        f'{arch}/include',
    ]
    
    for p in core_paths:
        full_path = base / p
        if full_path.exists():
            includes.append(str(full_path))
    
    # 添加所有组件的 include 目录
    for component in base.iterdir():
        if component.is_dir():
            inc = component / 'include'
            if inc.exists() and str(inc) not in includes:
                includes.append(str(inc))
    
    return includes


def build(req: dict) -> dict:
    """执行编译（复刻 Arduino）"""
    chip = req.get('chip', 'esp32c3')
    sources = req.get('sources', [])
    output = req.get('output', 'build/app.elf')
    cflags = req.get('cflags', [])
    ldflags = req.get('ldflags', [])
    includes = req.get('include', [])
    defines = req.get('defines', [])
    
    arch = CHIP_ARCH.get(chip, 'riscv')
    prefix = COMPILER_PREFIX[arch]
    
    toolchain_dir = get_toolchain_path(arch)
    if not toolchain_dir:
        return {'success': False, 'error': f'工具链未安装: {arch}, 请运行: zyrthi install esp32'}
    
    sdk_dir = get_sdk_dir(chip)
    if not sdk_dir:
        return {'success': False, 'error': f'SDK 未安装: {chip}, 请运行: zyrthi install esp32'}
    
    env = os.environ.copy()
    env['PATH'] = f"{toolchain_dir}:{env.get('PATH', '')}"
    
    output_path = Path(output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    
    # 展开源文件
    source_files = []
    for src in sources:
        src_path = Path(src)
        if src_path.is_dir():
            for f in src_path.rglob('*.c'):
                source_files.append(str(f))
        elif src_path.suffix == '.c':
            source_files.append(src)
        elif src_path.exists():
            source_files.append(src)
    
    if not source_files:
        return {'success': False, 'error': '没有找到源文件'}
    
    gcc = f'{prefix}gcc'
    cmd = [gcc]
    
    # Arduino 编译标志
    cmd.extend([
        '-march=rv32imc_zicsr_zifencei' if arch == 'riscv' else '-mlongcalls',
        '-std=gnu17',
        '-Os',
        '-ffunction-sections',
        '-fdata-sections',
        '-fno-jump-tables',
        '-fno-tree-switch-conversion',
        '-Wno-error=unused-function',
        '-Wno-error=unused-variable',
        '-Wno-error=deprecated-declarations',
        '-Wno-unused-parameter',
        '-Wno-sign-compare',
        '-gdwarf-4',
        '-nostartfiles',
    ])
    
    # 定义（Arduino 方式）
    cmd.extend(['-D', 'ESP_PLATFORM'])
    cmd.extend(['-D', f'IDF_VER="v5.3.2"'])
    cmd.extend(['-D', 'ARDUINO_ARCH_ESP32'])
    cmd.extend(['-D', f'CONFIG_IDF_TARGET_{chip.upper()}'])
    
    for d in defines:
        cmd.extend(['-D', d])
    
    cmd.extend(cflags)
    
    # 强制包含 sdkconfig.h
    sdkconfig_h = None
    for config_dir in ['dio_qspi', 'qio_qspi']:
        h = Path(sdk_dir) / config_dir / 'include' / 'sdkconfig.h'
        if h.exists():
            sdkconfig_h = str(h)
            break
    
    if sdkconfig_h:
        cmd.extend(['-include', sdkconfig_h])
    
    # 用户 include 路径
    for inc in includes:
        cmd.extend(['-I', inc])
    
    # ESP-IDF include 路径
    esp_includes = get_esp_idf_includes(sdk_dir, chip, arch)
    for inc in esp_includes:
        cmd.extend(['-I', inc])
    
    for src in source_files:
        cmd.append(src)
    
    # 链接标志（Arduino 方式）
    cmd.extend([
        '-nostartfiles',
        '--specs=nosys.specs',
        '-Wl,--gc-sections',
        '-Wl,--cref',
        '-Wl,--no-warn-rwx-segments',
        f'-Wl,--defsym=IDF_TARGET_{chip.upper()}=0',
    ])
    
    # 链接脚本
    ld_dir = Path(sdk_dir) / 'ld'
    for ld in ARDUINO_LINK_SCRIPTS:
        ld_file = ld_dir / ld
        if ld_file.exists():
            cmd.extend(['-T', str(ld_file)])
    
    cmd.extend(ldflags)
    
    # 库路径
    lib_dir = Path(sdk_dir) / 'lib'
    cmd.extend(['-L', str(lib_dir)])
    cmd.extend(['-L', str(ld_dir)])
    
    # 链接 Arduino 核心库（使用 --start-group/--end-group 解决循环依赖）
    cmd.append('-Wl,--start-group')
    cmd.extend(ARDUINO_CORE_LIBS)
    cmd.append('-Wl,--end-group')
    
    cmd.extend(['-o', output])
    
    try:
        print(f"编译: {len(cmd)} 参数", file=sys.stderr)
        result = subprocess.run(cmd, env=env, capture_output=True, text=True)
        
        if result.returncode != 0:
            return {'success': False, 'error': result.stderr or result.stdout}
        
        size = output_path.stat().st_size if output_path.exists() else 0
        return {'success': True, 'binary': output, 'size': size}
        
    except Exception as e:
        return {'success': False, 'error': str(e)}


def main():
    try:
        request = json.load(sys.stdin)
        response = build(request)
        print(json.dumps(response))
    except json.JSONDecodeError as e:
        print(json.dumps({'success': False, 'error': f'JSON 解析错误: {e}'}))
        sys.exit(1)
    except Exception as e:
        print(json.dumps({'success': False, 'error': str(e)}))
        sys.exit(1)


if __name__ == '__main__':
    main()