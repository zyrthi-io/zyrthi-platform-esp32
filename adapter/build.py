#!/usr/bin/env python3
"""
zyrthi build 适配器
读取 stdin JSON 请求，执行编译，输出 JSON 响应
"""

import json
import os
import subprocess
import sys
import shutil
from pathlib import Path
from typing import Optional

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


def get_compiler_path(arch: str) -> str:
    """获取编译器路径"""
    prefix = COMPILER_PREFIX.get(arch, 'riscv32-esp-elf-')
    # 检查工具链目录
    toolchain_dirs = [
        f'/opt/tools/{arch}-esp-elf/bin',
        '/opt/tools/xtensa-esp-elf/bin',
        '/opt/tools/riscv32-esp-elf/bin',
    ]
    
    for d in toolchain_dirs:
        gcc = os.path.join(d, f'{prefix}gcc')
        if os.path.exists(gcc):
            return d
    
    return ''


def build(req: dict) -> dict:
    """执行编译"""
    chip = req.get('chip', 'esp32c3')
    sources = req.get('sources', [])
    output = req.get('output', '/src/build/app.elf')
    cflags = req.get('cflags', [])
    ldflags = req.get('ldflags', [])
    includes = req.get('includes', [])
    defines = req.get('defines', [])
    
    # 确定架构
    arch = CHIP_ARCH.get(chip, 'riscv')
    prefix = COMPILER_PREFIX[arch]
    
    # 获取编译器路径
    toolchain_dir = get_compiler_path(arch)
    if not toolchain_dir:
        return {
            'success': False,
            'error': f'编译器未找到: {arch}',
        }
    
    # 设置 PATH
    env = os.environ.copy()
    env['PATH'] = f"{toolchain_dir}:{env.get('PATH', '')}"
    
    # 创建输出目录
    output_path = Path(output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    
    # 构建编译命令
    gcc = f'{prefix}gcc'
    
    cmd = [gcc]
    
    # 添加 C 标准标志
    cmd.extend(['-Os', '-Wall', '-g'])
    
    # 添加架构特定标志
    if arch == 'xtensa':
        cmd.append('-mlongcalls')
    
    # 添加用户自定义 flags
    cmd.extend(cflags)
    
    # 添加 defines
    for d in defines:
        cmd.extend(['-D', d])
    
    # 添加 include 路径
    for inc in includes:
        cmd.extend(['-I', inc])
    
    # 添加源文件
    for src in sources:
        cmd.append(src)
    
    # 添加链接标志
    cmd.extend(ldflags)
    
    # 输出文件
    cmd.extend(['-o', output])
    
    try:
        print(f"编译: {' '.join(cmd)}", file=sys.stderr)
        result = subprocess.run(
            cmd,
            env=env,
            capture_output=True,
            text=True,
            cwd='/src'
        )
        
        if result.returncode != 0:
            return {
                'success': False,
                'error': result.stderr or result.stdout,
            }
        
        # 获取文件大小
        size = output_path.stat().st_size if output_path.exists() else 0
        
        return {
            'success': True,
            'binary': output,
            'size': size,
        }
        
    except Exception as e:
        return {
            'success': False,
            'error': str(e),
        }


def main():
    """主函数"""
    try:
        # 读取 JSON 请求
        request = json.load(sys.stdin)
        
        # 执行���译
        response = build(request)
        
        # 输出 JSON 响应
        print(json.dumps(response))
        
    except json.JSONDecodeError as e:
        print(json.dumps({
            'success': False,
            'error': f'JSON 解析错误: {e}',
        }))
        sys.exit(1)
    except Exception as e:
        print(json.dumps({
            'success': False,
            'error': str(e),
        }))
        sys.exit(1)


if __name__ == '__main__':
    main()
