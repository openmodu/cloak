GOPATH_BIN := $(shell go env GOPATH)/bin
WIRE_VERSION := v0.7.0

.PHONY: all build test vet fmt wire wire-install crosscheck clean

all: fmt vet test build

build:
	go build -o bin/cloak ./cmd/cloak
	go build -o bin/cloakd ./cmd/cloakd

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# 修改 internal/bootstrap/provider.go 里的 ProviderSet 后重新生成注入代码
wire: wire-install
	$(GOPATH_BIN)/wire ./internal/bootstrap/

wire-install:
	@command -v $(GOPATH_BIN)/wire >/dev/null 2>&1 || go install github.com/google/wire/cmd/wire@$(WIRE_VERSION)

# 与上游 aifw 的 Zig 实现对拍中文地址融合，需要 zig 0.15.x
crosscheck:
	./scripts/crosscheck-zhaddr.sh

clean:
	rm -rf bin
