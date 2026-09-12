package regexrepo

import (
	"fmt"
	"regexp"
	"strings"
)

// 上游的正则引擎是 Rust 的 regex-automata，开了 unicode feature，`\b` 是
// **Unicode 词边界**：汉字算词字符，所以「A座1208室」里 1208 两侧都不成边界，
// `\b\d{4,8}\b` 匹配不上。Go 标准库 regexp 的 `\b` 只认 ASCII，会把同一段当成
// 验证码脱敏掉，中文文本上两边结果就此分叉。
//
// RE2 没有零宽断言，无法直接写出 Unicode 词边界，只能改成「吃掉一个分隔符」再用
// 捕获组取回真正的区间。为了让 PatternSpec.Pattern 能与上游逐字节保持一致，
// 这个改写放在编译期做，规则表里仍然写 `\b`。
const (
	wordClass    = `\p{L}\p{N}_`
	leftBoundary = `(?:\A|[^` + wordClass + `])`
	// 末尾用 \z 而不是 $，与上游把「串尾」视作非词字符的判定一致。
	rightBoundary = `(?:[^` + wordClass + `]|\z)`
)

// 形如 (?i)、(?is) 的纯标志组，只允许出现在表达式最前面。
var flagGroupRe = regexp.MustCompile(`^\(\?[imsU-]+\)`)

// translateWordBoundaries 把首尾的 `\b` 换成 RE2 可表达的等价写法，
// 并返回捕获组的偏移量：改写后整个核心表达式被包进第 1 组，原有组号顺延 1。
//
// 只处理首尾两处。表达式中间若还留着 `\b`，说明这条规则需要单独设计，直接报错，
// 好过让两边悄悄跑出不同结果。
func translateWordBoundaries(expr string) (translated string, groupShift int, err error) {
	flags := flagGroupRe.FindString(expr)
	core := expr[len(flags):]

	hasLeft := strings.HasPrefix(core, `\b`)
	if hasLeft {
		core = core[2:]
	}
	hasRight := endsWithWordBoundary(core)
	if hasRight {
		core = core[:len(core)-2]
	}
	if strings.Contains(core, `\b`) || strings.Contains(core, `\B`) {
		return "", 0, fmt.Errorf("表达式中间存在 \\b/\\B，无法翻译成 Unicode 词边界: %s", expr)
	}
	if !hasLeft && !hasRight {
		return expr, 0, nil
	}

	var b strings.Builder
	b.WriteString(flags)
	if hasLeft {
		b.WriteString(leftBoundary)
	}
	b.WriteString("(")
	b.WriteString(core)
	b.WriteString(")")
	if hasRight {
		b.WriteString(rightBoundary)
	}
	return b.String(), 1, nil
}

// endsWithWordBoundary 判断表达式是否以 `\b` 结尾。
// 要排除 `\\b`（转义过的反斜杠后跟字母 b）这种情况，因此要数一数结尾连续反斜杠的个数。
func endsWithWordBoundary(s string) bool {
	if !strings.HasSuffix(s, `\b`) {
		return false
	}
	n := 0
	for i := len(s) - 2; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}
