#!/usr/bin/env python3
"""
zyrthi monitor 适配器
读取 stdin JSON 请求，启动串口监控，输出 JSON 响应
"""

import json
import os
import subprocess
import sys
import glob
import serial
import serial.tools.list_ports
from pathlib import Path
from typing import Optional


def find_serial_port() -> Optional[str]:
    """自动查找串口"""
    # 优先使用串口列表
    ports = list(serial.tools.list_ports.comports())
    if ports:
        # 过滤掉蓝牙等非 USB 串口
        for port in ports:
            if 'USB' in port.description or 'UART' in port.description:
                return port.device
        # 返回第一个可用串口
        return ports[0].device
    
    # 备用：使用 glob 查找
    patterns = [
        '/dev/ttyUSB*',
        '/dev/ttyACM*',
        '/dev/cu.usbserial*',
        '/dev/cu.SLAB_USBtoUART*',
    ]
    
    for pattern in patterns:
        found = glob.glob(pattern)
        if found:
            return found[0]
    
    return None


def monitor(req: dict) -> dict:
    """启动串口监控"""
    port = req.get('port', '')
    baud = req.get('baud', 115200)
    timestamp = req.get('timestamp', False)
    
    # 自动查找串口
    if not port:
        port = find_serial_port()
        if not port:
            return {
                'success': False,
                'error': '未找到串口设备',
            }
    
    # 检查串口是否可用
    try:
        ser = serial.Serial(port, baud, timeout=1)
        ser.close()
    except serial.SerialException as e:
        return {
            'success': False,
            'error': f'无法打开串口 {port}: {e}',
        }
    
    # 使用 picocom 或 minicom
    # 优先使用 picocom
    monitor_cmd = None
    if os.path.exists('/usr/bin/picocom'):
        monitor_cmd = ['/usr/bin/picocom', '-b', str(baud), port]
    elif os.path.exists('/usr/bin/minicom'):
        monitor_cmd = ['/usr/bin/minicom', '-D', port, '-b', str(baud)]
    else:
        # 使用 Python 串口读取
        return {
            'success': True,
            'port': port,
            'baud': baud,
            'mode': 'python',
            'message': f'串口已连接: {port} @ {baud}',
        }
    
    try:
        # 启动监控（前台运行）
        os.execvp(monitor_cmd[0], monitor_cmd)
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
        
        # 启动监控
        response = monitor(request)
        
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
