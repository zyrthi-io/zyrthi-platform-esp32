# Makefile for zyrthi-docker-esp32

IMAGE ?= zyrthi/esp32
TAG ?= latest
RUNTIME ?= $(shell which podman 2>/dev/null || which docker 2>/dev/null || echo "podman")

.PHONY: build push test clean

build:
	$(RUNTIME) build -f Containerfile -t $(IMAGE):$(TAG) .

push:
	$(RUNTIME) push $(IMAGE):$(TAG)

test:
	@echo "Testing build adapter..."
	echo '{"chip":"esp32c3","sources":["/src/main.c"],"output":"/src/build/app.elf"}' | \
		$(RUNTIME) run --rm -i -v $(PWD)/test:/src $(IMAGE):$(TAG) build

clean:
	$(RUNTIME) image rm $(IMAGE):$(TAG) 2>/dev/null || true

# 开发目标
dev-shell:
	$(RUNTIME) run --rm -it -v $(PWD)/test:/src --entrypoint /bin/bash $(IMAGE):$(TAG)
