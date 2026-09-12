// Package textutil 提供与业务无关的文本工具：语言判定、字节偏移换算等。
package textutil

import "unicode"

// Script 是文本的书写系统判定结果。
type Script string

const (
	ScriptUnknown  Script = ""
	ScriptLatin    Script = "Latn"
	ScriptHans     Script = "Hans"
	ScriptHant     Script = "Hant"
	ScriptJapanese Script = "Jpan"
	ScriptKorean   Script = "Kore"
)

// hantOnly 收录一批只在繁体中出现、且在日常文本里高频的汉字，用来区分简繁。
//
// 这是启发式判定：命中即判繁体，一个都没命中则判简体。因此它对「通篇没有
// 简繁差异字的短文本」会判成简体——这种情况下两者本来也没有区别。
// 需要更高精度时应换成完整的简繁对照码表。
var hantOnly = map[rune]struct{}{}

func init() {
	const chars = "個們這裡與從對開關會學點國時長車馬鳥魚門東風雲龍區縣灣臺鄉鎮號樓層裝業產經濟財務資訊網絡電腦軟體讀寫發現實際貳參陸萬億謝請問題應該樣單雙歲圖書館銀證來為說話語職員專術機構織師傳統計劃備聽視覺醫療藥廠標準則規範圍繞認識別記憶檔夾複製貼稱碼誌總營運輸達郵遞憑據驗錯誤"
	for _, r := range chars {
		hantOnly[r] = struct{}{}
	}
}

// DetectScript 判定文本的书写系统。
//
// 判定顺序是有讲究的：先看假名与谚文这类**排他性**字符，再看汉字占比。
// 日文里大量使用汉字，只数汉字会把日文判成中文，进而套上中文地址规则、
// 选错 NER 模型、还做一次没有意义的简繁转换。
func DetectScript(text string) Script {
	var han, kana, hangul, latin, hant int
	for _, r := range text {
		switch {
		case isKana(r):
			kana++
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Han, r):
			han++
			if _, ok := hantOnly[r]; ok {
				hant++
			}
		case unicode.IsLetter(r):
			latin++
		}
	}

	// Require a meaningful proportion: an isolated annotation must not reroute
	// a Chinese document away from Chinese NER and address processing.
	total := han + kana + hangul + latin
	if kana > 0 && kana*4 >= total {
		return ScriptJapanese
	}
	if hangul > 0 && hangul*4 >= total {
		return ScriptKorean
	}

	if han == 0 {
		if latin > 0 {
			return ScriptLatin
		}
		return ScriptUnknown
	}
	// 中英混排时只要出现一定数量的汉字就按中文处理：中文规则对纯英文文本是
	// 空操作，误判为中文的代价远小于漏判。
	if han*10 < latin {
		return ScriptLatin
	}
	if hant > 0 {
		return ScriptHant
	}
	return ScriptHans
}

// isKana 判断是否是平假名或片假名（含半角片假名）。
func isKana(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		(r >= 0xFF66 && r <= 0xFF9D) // 半角片假名
}
