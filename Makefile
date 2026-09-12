GOPATH_BIN := $(shell go env GOPATH)/bin
WIRE_VERSION := v0.7.0

.PHONY: all build ci test test-race test-onnx vet fmt fmt-check wire wire-install clean

all: fmt vet test build

build:
	go build -o bin/cloak ./cmd/cloak
	go build -o bin/cloakd ./cmd/cloakd

test:
	go test ./...

test-race:
	go test -race ./...

test-onnx:
	@test -n "$(CLOAK_TEST_MODELS_DIR)" || (echo 'Set CLOAK_TEST_MODELS_DIR to the model root' >&2; exit 1)
	go test -tags cloak_onnx ./internal/bootstrap -run TestRealONNX -count=1 -v

# 本地跑一遍 CI 的全部检查
ci: fmt-check vet test test-race build

# 只检查不改写，CI 用
fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "以下文件未格式化："; echo "$$unformatted"; exit 1; fi

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
