GOPATH_BIN := $(shell go env GOPATH)/bin
WIRE_VERSION := v0.7.0

.PHONY: all build test vet fmt wire wire-install clean

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

clean:
	rm -rf bin
