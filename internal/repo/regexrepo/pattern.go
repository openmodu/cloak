package regexrepo

import (
	"fmt"
	"regexp"

	"github.com/openmodu/cloak/internal/types"
)

// PatternSpec 是一条正则规则的声明。
// 规则本身不带实体类型——类型由持有它的 Recognizer 决定。
//
// GroupIndex 为 0 时取整个匹配，大于 0 时只取该捕获组：用于「password: <值>」
// 这类靠前缀定位、但只该脱敏值本身的场景。
type PatternSpec struct {
	Name       string
	Pattern    string
	Score      float32
	GroupIndex int
}

// ValidateFunc 是匹配命中后的二次校验。
// 正则往往只是初筛，钱包地址、卡号这类还需要校验位检查才能定分数。
// 返回 ok=false 表示不作判断，沿用规则自带的分数；它只调整分数，不否决匹配——
// 要让一个匹配落选，返回一个低于采纳阈值的分数即可。
type ValidateFunc func(matched string) (score float32, ok bool)

type compiledPattern struct {
	spec PatternSpec
	re   *regexp.Regexp
	// group 是实际要取的捕获组号。词边界改写会把核心表达式包进一层捕获组，
	// 因此它未必等于 spec.GroupIndex。
	group int
}

func compile(spec PatternSpec) (compiledPattern, error) {
	expr, err := unicodeClasses(spec.Pattern)
	if err != nil {
		return compiledPattern{}, err
	}
	expr, shift, err := translateWordBoundaries(expr)
	if err != nil {
		return compiledPattern{}, fmt.Errorf("pattern %s: %w", spec.Name, err)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return compiledPattern{}, fmt.Errorf("compile pattern %s: %w", spec.Name, err)
	}
	group := spec.GroupIndex
	if shift > 0 {
		group += shift
	}
	if group > re.NumSubexp() {
		return compiledPattern{}, fmt.Errorf("pattern %s: group %d out of range (has %d)", spec.Name, spec.GroupIndex, re.NumSubexp()-shift)
	}
	return compiledPattern{spec: spec, re: re, group: group}, nil
}

// find 从 pos 起在**剩余文本**上找下一个匹配，取指定捕获组，然后把 pos 推到该组的结尾。
//
// 这里刻意对 text[pos:] 切片搜索，而不是用 FindAll 一次拿全部匹配。
// 两者语义不同：切片搜索会让 \b 这类锚点在切点处按「串首」重新判定边界，
// 而捕获组只前进到组尾，下一轮仍能匹配到被 FindAll 跳过的重叠区间。
func (p compiledPattern) find(text string, entityType types.EntityType, validate ValidateFunc) []types.Span {
	var out []types.Span
	g := 2 * p.group
	for pos := 0; pos <= len(text); {
		loc := p.re.FindStringSubmatchIndex(text[pos:])
		if loc == nil {
			break // 没有更多匹配
		}
		if g+1 >= len(loc) || loc[g] < 0 {
			break // 要取的捕获组没参与匹配，这条规则到此为止
		}
		s, e := pos+loc[g], pos+loc[g+1]

		score := p.spec.Score
		if validate != nil {
			if v, ok := validate(text[s:e]); ok {
				score = v
			}
		}
		out = append(out, types.Span{
			Type:   entityType,
			Start:  s,
			End:    e,
			Score:  score,
			Source: p.spec.Name,
		})

		if e > pos {
			pos = e
		} else {
			pos++
		}
	}
	return out
}
