// Package textutil 提供与业务无关的文本工具：语言判定、字节偏移换算等。
package textutil

import "unicode"

// Script 是文本的书写系统判定结果。
type Script string

const (
	ScriptUnknown Script = ""
	ScriptLatin   Script = "Latn"
	ScriptHans    Script = "Hans"
	ScriptHant    Script = "Hant"
)

// hantOnly 收录一批只在繁体中出现、且在日常文本里高频的汉字，用来把中文区分简繁。
// 这是启发式判定，只求在常见文本上稳定；需要更高精度时应换成 OpenCC 的完整码表。
var hantOnly = map[rune]struct{}{}

func init() {
	const chars = "個們這裡與從對開關會學點國時間長車馬鳥魚門東風雲龍區縣灣臺鄉鎮號樓層裝業產經濟財務資訊網絡電腦軟體讀寫發現實際壹貳參肆陸萬億謝請問題應該樣單雙萬歲圖書館銀行證券"
	for _, r := range chars {
		hantOnly[r] = struct{}{}
	}
}

// DetectScript 判定文本的书写系统。CJK 字符占比超过阈值即认定为中文，
// 再按繁体专用字是否出现区分简繁。
func DetectScript(text string) Script {
	var cjk, latin, hant int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			cjk++
			if _, ok := hantOnly[r]; ok {
				hant++
			}
		case unicode.IsLetter(r):
			latin++
		}
	}
	if cjk == 0 {
		if latin > 0 {
			return ScriptLatin
		}
		return ScriptUnknown
	}
	// 中英混排时只要出现一定数量的汉字就按中文处理，因为中文规则（地址融合）
	// 对纯英文文本是空操作，误判为中文的代价远小于漏判。
	if cjk*10 < latin {
		return ScriptLatin
	}
	if hant > 0 {
		return ScriptHant
	}
	return ScriptHans
}
