#!/usr/bin/env python3
"""
zyrthi monitor 适配器
读取 stdin JSON 请求，启动串口监控，输出 JSON 响应

架构:
  CLI (Go) --stdin JSON--> monitor.py --pyserial--> 串口
"""

import json
import glob
import platform
import serial
import serial.tools.list_ports
import sys
import time
import signal
from pathlib import Path
from typing import Optional


def find_serial_port() -> Optional[str]:
    """自动查找串口设备"""
    # 优先使用串口列表
    ports = list(serial.tools.list_ports.comports())
    if ports:
        # 过滤掉蓝牙等非 USB 串口
        for port in ports:
            if 'USB' in port.description or 'UART' in port.description or 'CH340' in port.description:
                return port.device
        # 返回第一个可用串口
        return ports[0].device
    
    # 备用：使用 glob 查找
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
                ser = serial.Serial(port)
                ser.close()
                return port
            except:
                continue
        return None
    else:
        patterns = ['/dev/ttyUSB*', '/dev/ttyACM*']
    
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
    
    # 打开串口并开始监控
    try:
        ser = serial.Serial(port, baud, timeout=0.1)
        print(f"串口监控已启动: {port} @ {baud}", file=sys.stderr)
        print("按 Ctrl+C 退出", file=sys.stderr)
        
        def signal_handler(sig, frame):
            ser.close()
            print("\n监控已停止", file=sys.stderr)
            sys.exit(0)
        
        signal.signal(signal.SIGINT, signal_handler)
        
        while True:
            if ser.in_waiting > 0:
                data = ser.read(ser.in_waiting)
                if timestamp:
                    print(f"[{time.strftime('%H:%M:%S')}] ", end='')
                sys.stdout.write(data.decode('utf-8', errors='replace'))
                sys.stdout.flush()
            time.sleep(0.01)
            
    except serial.SerialException as e:
        return {
            'success': False,
            'error': f'无法打开串口 {port}: {e}',
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