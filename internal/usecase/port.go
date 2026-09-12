// Package usecase 定义业务用例依赖的全部外部能力（端口）。
//
// 依赖倒置的支点在这里：接口由 usecase 侧声明，由 internal/repo 侧实现，
// usecase 永远不 import repo。子包（masker、restorer 等）通过这些接口拿能力。
package usecase

import (
	"context"

	"github.com/openmodu/cloak/internal/types"
)

// Recognizer 是唯一的识别抽象。正则、本地 NER 模型、远程 NER 服务都只是它的不同实现，
// masker 不关心底下是什么，也不关心接了几个。
type Recognizer interface {
	// Name 用于日志与 Span.Source 标注。
	Name() string
	// Recognize 返回文本中识别到的敏感区间，偏移必须是 UTF-8 字节偏移。
	Recognize(ctx context.Context, text string, lang types.Language) ([]types.Span, error)
}

// LangDetector 判定文本语言，决定是否启用中文地址融合等语言相关规则。
type LangDetector interface {
	Detect(text string) types.Language
}

// ConfigStore 持有可热更新的运行期配置，支撑 /api/config 这类免重启改配置的接口。
type ConfigStore interface {
	Mask() types.MaskConfig
	SetMask(types.MaskConfig)
}

// LLMClient 是下游大模型的最小抽象，实现方负责协议细节（OpenAI 兼容等）。
type LLMClient interface {
	Chat(ctx context.Context, req types.ChatRequest) (types.ChatReply, error)
}
