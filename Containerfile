# zyrthi-docker-esp32
# ESP32 平台容器镜像 - 简化版（用于测试）

FROM python:3.11-slim-bookworm

LABEL maintainer="zyrthi-io"
LABEL description="Zyrthi ESP32 Platform Container"
LABEL version="1.0.0"

# 安装基础工具
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    curl \
    ca-certificates \
    git \
    make \
    xz-utils \
    file \
    && rm -rf /var/lib/apt/lists/*

# 设置工作目录
WORKDIR /opt/zyrthi

# 安装 esptool
RUN pip install --no-cache-dir esptool==4.8.1 pyserial==3.5

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
