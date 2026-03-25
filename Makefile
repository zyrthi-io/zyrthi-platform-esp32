# Makefile for zyrthi-platform-esp32

IMAGE ?= zyrthi/esp32
TAG ?= latest
RUNTIME ?= $(shell which podman 2>/dev/null || which docker 2>/dev/null || echo "podman")

# 工具链缓存目录（持久化解压后的工具链）
TOOLCHAIN_CACHE ?= $(HOME)/.cache/zyrthi/esp32-toolchain

# 挂载参数（源码 + 工具链缓存）
VOLUMES = -v $(PWD)/test:/src
VOLUMES += -v $(TOOLCHAIN_CACHE):/opt/tools

.PHONY: build push install test clean

build:
	$(RUNTIME) build --squash -f Containerfile -t $(IMAGE):$(TAG) .

push:
	$(RUNTIME) push $(IMAGE):$(TAG)

# 安装工具链
install: $(TOOLCHAIN_CACHE)
	@echo "安装工具链..."
	echo '{"arch":"all"}' | $(RUNTIME) run --rm -i -v $(TOOLCHAIN_CACHE):/opt/tools $(IMAGE):$(TAG) install

# 安装单个架构
install-riscv: $(TOOLCHAIN_CACHE)
	echo '{"arch":"riscv"}' | $(RUNTIME) run --rm -i -v $(TOOLCHAIN_CACHE):/opt/tools $(IMAGE):$(TAG) install

install-xtensa: $(TOOLCHAIN_CACHE)
	echo '{"arch":"xtensa"}' | $(RUNTIME) run --rm -i -v $(TOOLCHAIN_CACHE):/opt/tools $(IMAGE):$(TAG) install

# 创建缓存目录
$(TOOLCHAIN_CACHE):
	mkdir -p $(TOOLCHAIN_CACHE)

# 测试 RISC-V (ESP32-C3/C6/H2)
test: $(TOOLCHAIN_CACHE)
	@echo "Testing RISC-V build (ESP32-C3)..."
	echo '{"chip":"esp32c3","sources":["/src/main.c"],"output":"/src/build/app.elf"}' | \
		$(RUNTIME) run --rm -i $(VOLUMES) $(IMAGE):$(TAG) build

# 测试 Xtensa (ESP32/S2/S3)
test-xtensa: $(TOOLCHAIN_CACHE)
	@echo "Testing Xtensa build (ESP32)..."
	echo '{"chip":"esp32","sources":["/src/main.c"],"output":"/src/build/app.elf"}' | \
		$(RUNTIME) run --rm -i $(VOLUMES) $(IMAGE):$(TAG) build

clean:
	$(RUNTIME) image rm $(IMAGE):$(TAG) 2>/dev/null || true

# 清理工具链缓存
clean-cache:
	rm -rf $(TOOLCHAIN_CACHE)

# 开发目标
dev-shell: $(TOOLCHAIN_CACHE)
	$(RUNTIME) run --rm -it $(VOLUMES) --entrypoint /bin/bash $(IMAGE):$(TAG)

# 显示配置
info:
	@echo "Runtime: $(RUNTIME)"
	@echo "Image: $(IMAGE):$(TAG)"
	@echo "Cache: $(TOOLCHAIN_CACHE)"