# zyrthi-platform-esp32

ESP32 系列芯片平台配置与烧录插件。

## 支持的芯片

- ESP32
- ESP32-S2
- ESP32-S3
- ESP32-C3
- ESP32-C6
- ESP32-H2

## 目录结构

```
zyrthi-platform-esp32/
├── platform.yaml      # 平台配置
├── plugin/            # 烧录插件源码（标准 Go WASM）
│   ├── protocol.go    # 协议层（可测试）
│   ├── protocol_test.go
│   ├── main_wasm.go   # WASM 入口
│   └── Makefile
├── releases/          # 编译产物（GitHub Release）
└── README.md
```

## 安装

```bash
zyrthi platform install esp32
```

## 编译插件

使用标准 Go 1.24+：

```bash
cd plugin
make
```

## 发布

1. 编译插件：`make`
2. 创建 GitHub Release
3. 上传 `esp32-flash.wasm`
4. 更新 `platform.yaml` 中的插件 URL

## 维护者

当前由 zyrthi-io 维护，后续将转交给乐鑫官方。