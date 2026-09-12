//go:build wireinject

// 本文件只在 wire 代码生成时参与编译，生成结果见 wire_gen.go。
// 修改 ProviderSet 后执行 `make wire` 重新生成。

package bootstrap

import (
	"github.com/google/wire"

	httptransport "github.com/openmodu/cloak/internal/transport/http"
)

// InitApp 装配脱敏/还原两条流水线，CLI 与测试用它。
func InitApp(cfg Config) (*App, error) {
	wire.Build(ProviderSet)
	return nil, nil
}

// InitServer 在此之上装配 HTTP 服务，cloakd 用它。
func InitServer(cfg Config) (*httptransport.Server, error) {
	wire.Build(ServerSet)
	return nil, nil
}
