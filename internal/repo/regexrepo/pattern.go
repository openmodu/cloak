package regexrepo

import (
	"fmt"
	"regexp"

	"github.com/openmodu/cloak/internal/types"
)

// PatternSpec 是一条正则规则的声明，对应上游 RegexRecognizer.PatternSpec。
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

// ValidateFunc 对应上游的 ValidateResultFn（返回 ?f32）。
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
	expr, shift, err := translateWordBoundaries(spec.Pattern)
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

// find 复刻上游的扫描循环：从 pos 起在**剩余文本**上找下一个匹配，取指定捕获组，
// 然后把 pos 推到该组的结尾。
//
// 注意这里刻意对 text[pos:] 切片搜索，而不是用 FindAll 一次拿全部匹配：上游
// 传给 Rust 的也是 &hay[start..]，这会让 \b 之类的锚点在切点处重新判定边界，
// 换成 FindAll 语义就不一样了。
func (p compiledPattern) find(text string, entityType types.EntityType, validate ValidateFunc) []types.Span {
	var out []types.Span
	g := 2 * p.group
	for pos := 0; pos <= len(text); {
		loc := p.re.FindStringSubmatchIndex(text[pos:])
		if loc == nil {
			break // 没有更多匹配
		}
		if g+1 >= len(loc) || loc[g] < 0 {
			break // 要取的捕获组未参与匹配，与上游 rc==0 的处理一致
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
