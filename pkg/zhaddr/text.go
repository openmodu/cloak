package zhaddr

// 以下一组工具逐个对应上游 core/merge_zh_addr.zig 顶部的同名函数。
// 全部按字节偏移工作：Go 的 string 本身就是 UTF-8 字节序列，与上游的 []const u8 同构。

// isASCIILight 是「轻分隔符」：空白与半角逗号。地址各段之间允许夹这些字符。
func isASCIILight(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == ','
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// utf8CpLenAt 返回 pos 处字符的字节长度，越界或非法时返回 1，与上游一致。
func utf8CpLenAt(text string, pos int) int {
	if pos >= len(text) {
		return 1
	}
	b := text[pos]
	switch {
	case b < 0x80:
		return 1
	case b&0xE0 == 0xC0:
		if pos+1 <= len(text) {
			return 2
		}
	case b&0xF0 == 0xE0:
		if pos+2 <= len(text) {
			return 3
		}
	case b&0xF8 == 0xF0:
		if pos+3 <= len(text) {
			return 4
		}
	}
	return 1
}

// utf8PrevCpStart 回退到 pos 之前那个字符的起始字节。
func utf8PrevCpStart(text string, pos int) int {
	if pos == 0 {
		return 0
	}
	p := pos - 1
	for p > 0 && text[p]&0xC0 == 0x80 {
		p--
	}
	return p
}

// matchToken 判断 pos 处是否正好是 token。
func matchToken(text string, pos int, token string) bool {
	if pos > len(text) || pos+len(token) > len(text) {
		return false
	}
	return text[pos:pos+len(token)] == token
}

// heavySeps 是「重分隔符」：跨过它们通常意味着换了一个语义片段，
// 地址向右扩展到达隐私阈值后就不再越过它们。
var heavySeps = []string{"。", "！", "？", "；", "：", "、", "（", "）", "/", "\\", "|"}

func heavySepAt(text string, pos int) int {
	for _, s := range heavySeps {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

// countCharsBetween 统计 [a,b) 之间的字符数（按 UTF-8 首字节计数）。
func countCharsBetween(text string, a, b int) int {
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	count := 0
	for i := lo; i < hi; i++ {
		if text[i]&0xC0 != 0x80 {
			count++
		}
	}
	return count
}

// onlyLightBetween 判断两段之间是否只隔着不超过 maxChars 个轻分隔符。
func onlyLightBetween(text string, aEnd, bStart, maxChars int) bool {
	chars := 0
	for p := aEnd; p < bStart; p++ {
		if !isASCIILight(text[p]) {
			return false
		}
		if text[p]&0xC0 != 0x80 {
			chars++
		}
		if chars > maxChars {
			return false
		}
	}
	return true
}

// nearCharsBetween 只看距离，不限制中间是什么字符。
func nearCharsBetween(text string, aEnd, bStart, maxChars int) bool {
	return countCharsBetween(text, aEnd, bStart) <= maxChars
}

// endsWithAny 判断 [start,end) 是否以 toks 里任意一个结尾。
func endsWithAny(text string, start, end int, toks []string) bool {
	if end <= start {
		return false
	}
	for _, t := range toks {
		if end >= start+len(t) && matchToken(text, end-len(t), t) {
			return true
		}
	}
	return false
}
