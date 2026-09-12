// Package langrepo 提供语言判定能力，实现 usecase.LangDetector。
package langrepo

import (
	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/pkg/textutil"
)

// Detector 目前基于字符分布做启发式判定，够用且零依赖。
// 后续要提高精度（例如接 lingua-go）只需在这里换实现，usecase 侧无感。
type Detector struct{}

func New() *Detector { return &Detector{} }

func (d *Detector) Detect(text string) types.Language {
	switch textutil.DetectScript(text) {
	case textutil.ScriptHans:
		return types.LangZhHans
	case textutil.ScriptHant:
		return types.LangZhHant
	case textutil.ScriptJapanese:
		return types.LangJapanese
	case textutil.ScriptKorean:
		return types.LangKorean
	case textutil.ScriptLatin:
		return types.LangEnglish
	default:
		return types.LangUnknown
	}
}
