# zyrthi-flash-plugins-esp32 Makefile

GO ?= go
PLUGIN_NAME = esp32-flash
OUTPUT = $(PLUGIN_NAME).wasm

.PHONY: all clean install test

all: $(OUTPUT)

$(OUTPUT):
	GOOS=wasip1 GOARCH=wasm $(GO) build -o $(OUTPUT) .

clean:
	rm -f $(OUTPUT)

install: $(OUTPUT)
	@mkdir -p ~/.zyrthi/plugins
	cp $(OUTPUT) ~/.zyrthi/plugins/
	@echo "Installed to ~/.zyrthi/plugins/$(OUTPUT)"

test:
	$(GO) test -v -race -coverprofile=coverage.out .

# 开发测试
test-wasm: $(OUTPUT)
	@echo "Checking WASM exports..."
	@wasm-objdump -x $(OUTPUT) 2>/dev/null | grep -E "export.*\(func\)" || echo "Install wabt for wasm-objdump"