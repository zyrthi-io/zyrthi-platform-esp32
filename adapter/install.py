#!/usr/bin/env python3
"""
zyrthi install 适配器
安装/解压工具链和 SDK
"""

import json
import os
import sys
import tarfile
from pathlib import Path


def extract_tar_xz(archive: Path, target: Path) -> dict:
    """解压 tar.xz 文件"""
    try:
        target.mkdir(parents=True, exist_ok=True)
        
        with tarfile.open(archive, 'r:xz') as tar:
            for member in tar.getmembers():
                if member.isdir():
                    member.mode = 0o755
                elif '/bin/' in member.name or '/libexec/' in member.name or '/ld/' in member.name:
                    member.mode = 0o755
                else:
                    member.mode = 0o644
                tar.extract(member, target)
        
        return {'success': True}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def install_toolchain(arch: str, tools_dir: Path) -> dict:
    """解压工具链"""
    archive = Path(f'/opt/zyrthi/toolchain/{arch}-esp-elf.tar.xz')
    target_dir = tools_dir / 'toolchain' / arch
    
    # 检查是否已安装
    if target_dir.exists():
        gcc_name = 'riscv32-esp-elf-gcc' if arch == 'riscv32' else 'xtensa-esp-elf-gcc'
        gcc = target_dir / 'bin' / gcc_name
        if gcc.exists():
            return {'success': True, 'message': f'工具链已安装: {arch}', 'cached': True}
    
    if not archive.exists():
        return {'success': False, 'error': f'工具链压缩包不存在: {archive}'}
    
    print(f"解压工具链: {arch} ...", file=sys.stderr)
    result = extract_tar_xz(archive, tools_dir / 'toolchain')
    
    if not result['success']:
        return result
    
    print(f"工具链解压完成: {arch}", file=sys.stderr)
    
    # 验证（解压后目录名为 arch）
    if target_dir.exists():
        size = sum(f.stat().st_size for f in target_dir.rglob('*') if f.is_file())
        return {
            'success': True,
            'message': f'工具链安装成功: {arch}',
            'size_mb': size // (1024 * 1024),
            'cached': False,
        }
    else:
        return {'success': False, 'error': f'解压后目录不存在: {target_dir}'}


def install_idf(tools_dir: Path) -> dict:
    """解压 ESP-IDF"""
    archive = Path('/opt/zyrthi/sdk/esp-idf.tar.xz')
    # IDF 解压到 tools_dir 根目录，检查 components 存在即可
    components_dir = tools_dir / 'components'
    
    # 检查是否已安装
    if components_dir.exists() and (tools_dir / 'tools').exists():
        return {'success': True, 'message': 'ESP-IDF 已安装', 'cached': True}
    
    if not archive.exists():
        return {'success': False, 'error': f'ESP-IDF 压缩包不存在: {archive}'}
    
    print("解压 ESP-IDF ...", file=sys.stderr)
    result = extract_tar_xz(archive, tools_dir)
    
    if not result['success']:
        return result
    
    print("ESP-IDF 解压完成", file=sys.stderr)
    
    if components_dir.exists():
        size = sum(f.stat().st_size for f in tools_dir.rglob('*') if f.is_file())
        return {
            'success': True,
            'message': 'ESP-IDF 安装成功',
            'size_mb': size // (1024 * 1024),
            'cached': False,
        }
    else:
        return {'success': False, 'error': f'解压后目录不存在: {components_dir}'}


def install_python_env(tools_dir: Path) -> dict:
    """解压 Python 环境"""
    archive = Path('/opt/zyrthi/python-env/python-env.tar.xz')
    target_dir = tools_dir / 'python-env'
    
    if target_dir.exists() and (target_dir / 'bin').exists():
        return {'success': True, 'message': 'Python 环境已安装', 'cached': True}
    
    if not archive.exists():
        return {'success': False, 'error': f'Python 环境压缩包不存在: {archive}'}
    
    print("解压 Python 环境 ...", file=sys.stderr)
    result = extract_tar_xz(archive, tools_dir)
    
    if not result['success']:
        return result
    
    print("Python 环境解压完成", file=sys.stderr)
    
    if target_dir.exists():
        size = sum(f.stat().st_size for f in target_dir.rglob('*') if f.is_file())
        return {
            'success': True,
            'message': 'Python 环境安装成功',
            'size_mb': size // (1024 * 1024),
            'cached': False,
        }
    else:
        return {'success': False, 'error': f'解压后目录不存在: {target_dir}'}


def install_debug_tools(tools_dir: Path) -> dict:
    """解压调试工具 (openocd, gdb, ulp)"""
    archive = Path('/opt/zyrthi/debug/debug-tools.tar.xz')
    # 调试工具解压到 tools_dir 根目录
    openocd_dir = tools_dir / 'openocd-esp32'
    
    if openocd_dir.exists():
        return {'success': True, 'message': '调试工具已安装', 'cached': True}
    
    if not archive.exists():
        return {'success': False, 'error': f'调试工具压缩包不存在: {archive}'}
    
    print("解压调试工具 ...", file=sys.stderr)
    result = extract_tar_xz(archive, tools_dir)
    
    if not result['success']:
        return result
    
    print("调试工具解压完成", file=sys.stderr)
    
    if openocd_dir.exists():
        size = sum(f.stat().st_size for f in openocd_dir.rglob('*') if f.is_file())
        return {
            'success': True,
            'message': '调试工具安装成功',
            'size_mb': size // (1024 * 1024),
            'cached': False,
        }
    else:
        return {'success': False, 'error': f'解压后目录不存在: {openocd_dir}'}


def install(req: dict) -> dict:
    """安装所有组件"""
    tools_dir = Path(os.environ.get('ZYRTHI_TOOLS_DIR', '/opt/tools'))
    
    results = []
    messages = []
    
    # 安装工具链
    for arch in ['xtensa', 'riscv32']:
        result = install_toolchain(arch, tools_dir)
        results.append(result)
        if result.get('message'):
            messages.append(result['message'])
        elif result.get('error'):
            messages.append(result['error'])
    
    # 安装 ESP-IDF
    idf_result = install_idf(tools_dir)
    results.append(idf_result)
    if idf_result.get('message'):
        messages.append(idf_result['message'])
    elif idf_result.get('error'):
        messages.append(idf_result['error'])
    
    # 安装 Python 环境
    py_result = install_python_env(tools_dir)
    results.append(py_result)
    if py_result.get('message'):
        messages.append(py_result['message'])
    elif py_result.get('error'):
        messages.append(py_result['error'])
    
    # 安装调试工具
    debug_result = install_debug_tools(tools_dir)
    results.append(debug_result)
    if debug_result.get('message'):
        messages.append(debug_result['message'])
    elif debug_result.get('error'):
        messages.append(debug_result['error'])
    
    success = all(r['success'] for r in results)
    
    return {
        'success': success,
        'messages': messages,
        'tools_dir': str(tools_dir),
        'details': results,
    }


def main():
    try:
        request = json.load(sys.stdin)
        response = install(request)
        print(json.dumps(response))
    except Exception as e:
        print(json.dumps({'success': False, 'error': str(e)}))
        sys.exit(1)


if __name__ == '__main__':
    main()
