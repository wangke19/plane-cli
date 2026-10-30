# plane-cli — 构建与开发任务
#
# 常用目标：
#   make build     编译到 bin/plane
#   make test      运行全部测试
#   make vet       go vet 静态检查
#   make fmt       gofmt 格式化
#   make install   安装到 ~/.local/bin/plane-go
#   make clean     清理构建产物

GO      ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BIN     := bin/plane
INSTALL_TO ?= $(HOME)/.local/bin/plane-go

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet fmt install clean all

all: vet test build

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/plane

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

install: build
	install -d $(dir $(INSTALL_TO))
	install -m 0755 $(BIN) $(INSTALL_TO)
	@echo "已安装到 $(INSTALL_TO)"

clean:
	rm -rf bin
