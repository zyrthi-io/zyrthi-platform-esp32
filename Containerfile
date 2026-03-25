# zyrthi-platform-esp32
# ESP32 平台容器镜像 - 压缩包版（参考 Arduino）
# 只需要 Linux 版本

FROM python:3.11-slim-bookworm

LABEL maintainer="zyrthi-io"
LABEL description="Zyrthi ESP32 Platform Container"
LABEL version="2.0.0"

# 使用阿里云镜像加速
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources

# 安装基础工具
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    make \
    file \
    xz-utils \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# 设置工作目录
WORKDIR /opt/zyrthi

# 使用清华 PyPI 镜像安装 esptool
RUN pip install --no-cache-dir -i https://pypi.tuna.tsinghua.edu.cn/simple \
    esptool==4.8.1 pyserial==3.5

# 创建目录（存放压缩包）
RUN mkdir -p /opt/zyrthi/toolchain /opt/zyrthi/sdk

# 下载压缩的工具链包（不解压，install 时解压）
RUN curl -fsSL "https://dl.espressif.cn/github_assets/espressif/crosstool-NG/releases/download/esp-14.2.0_20260121/riscv32-esp-elf-14.2.0_20260121-x86_64-linux-gnu.tar.xz" \
    -o /opt/zyrthi/toolchain/riscv32-esp-elf.tar.xz

RUN curl -fsSL "https://dl.espressif.cn/github_assets/espressif/crosstool-NG/releases/download/esp-14.2.0_20260121/xtensa-esp-elf-14.2.0_20260121-x86_64-linux-gnu.tar.xz" \
    -o /opt/zyrthi/toolchain/xtensa-esp-elf.tar.xz

# 下载 Arduino ESP32 SDK 并压缩（install 时解压）
# 使用 curl 重试选项处理网络问题
RUN curl -fsSL --retry 3 --retry-delay 5 --max-time 600 \
    "https://dl.espressif.com/AE/esp-arduino-libs/esp32-3.1.1.zip" \
    -o /tmp/esp32-sdk.zip && \
    cd /opt/zyrthi/sdk && \
    unzip -q /tmp/esp32-sdk.zip && \
    tar -cJf esp32-sdk.tar.xz arduino-esp32-libs-all-release_v5.3-cfea4f7c98 && \
    rm -rf arduino-esp32-libs-all-release_v5.3-cfea4f7c98 /tmp/esp32-sdk.zip

# 复制适配器脚本
COPY adapter/ /opt/zyrthi/adapter/

# 设置脚本权限
RUN chmod +x /opt/zyrthi/adapter/*.sh /opt/zyrthi/adapter/*.py

# 设置环境变量
ENV PATH="/opt/zyrthi/adapter:${PATH}"

# 设置工作目录为挂载点
WORKDIR /src

# 设置入口点
ENTRYPOINT ["/opt/zyrthi/adapter/entrypoint.sh"]
