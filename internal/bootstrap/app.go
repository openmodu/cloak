// Package bootstrap 负责装配：把 repo 层的具体实现注入 usecase 层的接口。
//
// 这是整个工程里唯一同时认识 usecase 与 repo 的地方，依赖注入由 google/wire
// 在编译期生成（见 wire.go 与 wire_gen.go），运行期没有反射与容器。
package bootstrap

import (
	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/internal/usecase/masker"
	"github.com/openmodu/cloak/internal/usecase/restorer"
)

// App 是装配完成的应用对象，transport 层从它取用例。
//
// 一个 App 持有配置好的脱敏与还原两条流水线。
// 还原凭据由调用方自己保管（HTTP 上就是随响应回传的 maskMeta），服务端不留存——
// 凭据里含有原文，不落地就不存在被翻出来的风险。
type App struct {
	Masker   *masker.Masker
	Restorer *restorer.Restorer
	Conf     usecase.ConfigStore
}

func NewApp(m *masker.Masker, r *restorer.Restorer, conf usecase.ConfigStore) *App {
	return &App{Masker: m, Restorer: r, Conf: conf}
}

// Config 是装配需要的外部配置，由 cmd 层按「命令行 > 环境变量 > 配置文件」解析好后传进来。
type Config struct {
	// ModelsDir 是 NER 模型根目录。留空、目录不存在、或二进制没带 ONNX 支持时，
	// 识别退化成「只有正则」，服务照常可用。
	ModelsDir string
	// ENModelID / ZHModelID 是两个 NER 模型在 ModelsDir 下的相对路径。
	// 留空则用 DefaultENModelID / DefaultZHModelID。
	// 做成可配置是因为模型仓库会改名、下架，或者使用者本来就想换成自己微调的模型。
	ENModelID string
	ZHModelID string
	// APIKeyFile 是 LLM 的 API key 文件路径。留空则不接入大模型，
	// /api/call 会明确回 503，而脱敏/还原接口照常可用。
	APIKeyFile string
	// HTTPAPIKey 非空时启用 Authorization 校验。
	HTTPAPIKey string
	// APIKeyDir 启用「请求级 apiKeyFile」并把它限制在这个目录之内。
	//
	// 留空则该功能关闭。这是安全默认值：开启后请求能指定服务端去读哪个文件，
	// 不加限制就等于把任意本地文件读取的能力交给了调用方。
	APIKeyDir  string
	MaskConfig map[string]*bool
	// AddressFallback 为 nil 时取默认值（开启）。见 masker.WithAddressFallback。
	AddressFallback *bool
	Temperature     float32
}

// addressFallbackOrDefault 在未显式配置时返回默认值。
func (c Config) addressFallbackOrDefault() bool {
	if c.AddressFallback == nil {
		return true
	}
	return *c.AddressFallback
}
