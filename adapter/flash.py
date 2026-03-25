#!/usr/bin/env python3
"""
zyrthi flash 适配器
读取 stdin JSON 请求，执行烧录，输出 JSON 响应

架构:
  CLI (Go) --stdin JSON--> flash.py --调用--> esptool.py --pyserial--> 串口
"""

import json
import glob
import platform
import subprocess
import sys
from pathlib import Path
from typing import Optional

# 芯片配置
CHIP_CONFIG = {
    'esp32': {
        'chip_flag': 'esp32',
        'entry_addr': '0x1000',
    },
    'esp32s2': {
        'chip_flag': 'esp32s2',
        'entry_addr': '0x1000',
    },
    'esp32s3': {
        'chip_flag': 'esp32s3',
        'entry_addr': '0x0',
    },
    'esp32c3': {
        'chip_flag': 'esp32c3',
        'entry_addr': '0x0',
    },
    'esp32c6': {
        'chip_flag': 'esp32c6',
        'entry_addr': '0x0',
    },
    'esp32h2': {
        'chip_flag': 'esp32h2',
        'entry_addr': '0x0',
    },
}


def find_serial_port() -> Optional[str]:
    """自动查找串口设备"""
    system = platform.system()
    
    if system == 'Linux':
        patterns = ['/dev/ttyUSB*', '/dev/ttyACM*']
    elif system == 'Darwin':  # macOS
        patterns = ['/dev/cu.usb*', '/dev/cu.wch*', '/dev/cu.SLAB_USBtoUART*']
    elif system == 'Windows':
        # Windows 下尝试 COM1-COM20
        for i in range(1, 21):
            port = f'COM{i}'
            try:
                import serial
                ser = serial.Serial(port)
                ser.close()
                return port
            except:
                continue
        return None
    else:
        patterns = ['/dev/ttyUSB*', '/dev/ttyACM*']
    
    for pattern in patterns:
        ports = glob.glob(pattern)
        if ports:
            return ports[0]
    
    return None


def flash(req: dict) -> dict:
    """执行烧录"""
    chip = req.get('chip', 'esp32c3')
    port = req.get('port', '')
    baud = req.get('baud', 921600)
    firmware = req.get('firmware', 'build/app.bin')
    erase = req.get('erase', False)
    verify = req.get('verify', False)
    
    # 获取芯片配置
    config = CHIP_CONFIG.get(chip, CHIP_CONFIG['esp32c3'])
    
    # 检查固件文件
    firmware_path = Path(firmware)
    if not firmware_path.exists():
        # 尝试 .bin 文件
        bin_path = firmware_path.with_suffix('.bin')
        if bin_path.exists():
            firmware = str(bin_path)
        else:
            return {
                'success': False,
                'error': f'固件文件不存在: {firmware}',
            }
    
    # 自动查找串口
    if not port:
        port = find_serial_port()
        if not port:
            return {
                'success': False,
                'error': '未找到串口设备',
            }
    
    # 擦除
    if erase:
        erase_cmd = [
            'esptool.py',
            '--chip', config['chip_flag'],
            '--port', port,
            '--baud', str(baud),
            'erase_flash',
        ]
        try:
            print(f"擦除: {' '.join(erase_cmd)}", file=sys.stderr)
            result = subprocess.run(erase_cmd, capture_output=True, text=True)
            if result.returncode != 0:
                return {
                    'success': False,
                    'error': f'擦除失败: {result.stderr}',
                }
        except FileNotFoundError:
            return {
                'success': False,
                'error': 'esptool.py 未安装，请运行: pip install esptool',
            }
        except Exception as e:
            return {
                'success': False,
                'error': str(e),
            }
    
    # 写入固件
    cmd = [
        'esptool.py',
        '--chip', config['chip_flag'],
        '--port', port,
        '--baud', str(baud),
    ]
    
    if verify:
        cmd.append('--verify')
    
    cmd.extend([
        'write_flash',
        config['entry_addr'],
        firmware,
    ])
    
    try:
        print(f"烧录: {' '.join(cmd)}", file=sys.stderr)
        result = subprocess.run(cmd, capture_output=True, text=True)
        
        if result.returncode != 0:
            return {
                'success': False,
                'error': result.stderr or result.stdout,
                'port': port,
            }
        
        return {
            'success': True,
            'port': port,
            'chip': chip,
            'firmware': firmware,
        }
        
    except FileNotFoundError:
        return {
            'success': False,
            'error': 'esptool.py 未安装，请运行: pip install esptool',
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
        
        # 执行烧录
        response = flash(request)
        
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