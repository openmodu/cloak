package regexrepo

import (
	"fmt"
	"regexp"
	"strings"
)

// Go 标准库 regexp 的 `\b` 只认 **ASCII** 词边界，这在中文文本上会出事：
// 「A座1208室」里 1208 的两侧都是汉字，ASCII 判定认为成边界，于是
// `\b\d{4,8}\b` 会把它当验证码脱敏掉——而按 Unicode 判定汉字算词字符，
// 这里根本不该成边界。
//
// RE2 没有零宽断言，无法直接写出 Unicode 词边界，只能改成「吃掉一个分隔符」
// 再用捕获组取回真正的区间。为了让规则表里仍然能直观地写 `\b`，
// 这个改写放在编译期做。
const (
	wordClass    = `\p{L}\p{N}_`
	leftBoundary = `(?:\A|[^` + wordClass + `])`
	// 末尾用 \z 而不是 $：$ 会在换行前也成立，这里要的是真正的串尾。
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
