package types

import "strings"

// Language 是 BCP-47 风格的语言标签，例如 "en"、"zh-Hans"、"zh-Hant"。
type Language string

const (
	LangUnknown Language = ""
	LangEnglish Language = "en"
	LangZhHans  Language = "zh-Hans"
	LangZhHant  Language = "zh-Hant"
)

// IsChinese 判断是否属于中文语系，中文地址融合等规则依赖它。
func (l Language) IsChinese() bool {
	s := strings.ToLower(string(l))
	return s == "zh" || strings.HasPrefix(s, "zh-") || strings.HasPrefix(s, "zh_")
}
