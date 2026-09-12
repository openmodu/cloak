// Package regexrepo 提供基于正则的识别器，实现 usecase.Recognizer。
//
// 与上游一致：一个 Recognizer 只负责一种实体类型，持有该类型的规则集和一个可选的
// 二次校验函数；整套识别能力由 NewDefaultSet 按实体枚举顺序建出的一组实例组成。
package regexrepo

import (
	"context"

	"github.com/openmodu/cloak/internal/types"
)

// Recognizer 对应上游的 RegexRecognizer，一个实例只产出一种实体类型的区间。
type Recognizer struct {
	entityType types.EntityType
	patterns   []compiledPattern
	validate   ValidateFunc
}

// New 构造某个实体类型的识别器。
//
// 与上游同构：内置规则总是生效，extra 里与内置重复的规则会被跳过，
// 因此传 nil 就等价于「只用内置规则」。
func New(entityType types.EntityType, extra []PatternSpec, validate ValidateFunc) (*Recognizer, error) {
	preset := PresetSpecsFor(entityType)
	specs := make([]PatternSpec, 0, len(preset)+len(extra))
	specs = append(specs, preset...)

	seen := make(map[string]struct{}, len(preset))
	for _, s := range preset {
		seen[s.Pattern] = struct{}{}
	}
	for _, s := range extra {
		if _, dup := seen[s.Pattern]; dup {
			continue
		}
		seen[s.Pattern] = struct{}{}
		specs = append(specs, s)
	}

	patterns := make([]compiledPattern, 0, len(specs))
	for _, s := range specs {
		c, err := compile(s)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, c)
	}
	return &Recognizer{entityType: entityType, patterns: patterns, validate: validate}, nil
}

// NewDefaultSet 按实体类型枚举顺序建出全套内置识别器。
// 顺序会影响后续排序中完全同分同范围区间的取舍，因此必须与上游一致。
func NewDefaultSet() ([]*Recognizer, error) {
	types_ := types.AllEntityTypes()
	out := make([]*Recognizer, 0, len(types_))
	for _, t := range types_ {
		r, err := New(t, nil, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (r *Recognizer) Name() string { return "regex:" + r.entityType.String() }

// EntityType 返回该识别器负责的实体类型。
func (r *Recognizer) EntityType() types.EntityType { return r.entityType }

// Recognize 逐条规则扫描全文。规则之间产生的重叠交给上层统一消解，这里不做取舍。
func (r *Recognizer) Recognize(_ context.Context, text string, _ types.Language) ([]types.Span, error) {
	if text == "" || len(r.patterns) == 0 {
		return nil, nil
	}
	var out []types.Span
	for _, p := range r.patterns {
		out = append(out, p.find(text, r.entityType, r.validate)...)
	}
	return out, nil
}
