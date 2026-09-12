// Package masker 实现脱敏用例：编排识别器、规整区间、产出占位符文本与还原凭据。
package masker

import (
	"context"
	"fmt"
	"strings"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/pkg/placeholder"
	"github.com/openmodu/cloak/pkg/span"
	"github.com/openmodu/cloak/pkg/zhaddr"
)

// DefaultMinScore 是区间被采纳的最低置信度。
// 调低会增加误伤，调高会漏标；改之前先跑一遍金样本回归。
const DefaultMinScore float32 = 0.5

// Masker 是脱敏用例对象。它只持有接口，不关心识别能力来自正则还是模型。
type Masker struct {
	recognizers []usecase.Recognizer
	detector    usecase.LangDetector
	conf        usecase.ConfigStore
	minScore    float32
}

type Option func(*Masker)

// WithRecognizers 追加识别器。顺序会影响完全同分同范围区间的取舍，别随意调整。
func WithRecognizers(rs ...usecase.Recognizer) Option {
	return func(m *Masker) { m.recognizers = append(m.recognizers, rs...) }
}

// WithLangDetector 设置语言判定器，不设置时一律按未知语言处理。
func WithLangDetector(d usecase.LangDetector) Option {
	return func(m *Masker) { m.detector = d }
}

// WithConfigStore 设置脱敏开关来源，不设置时使用默认配置。
func WithConfigStore(c usecase.ConfigStore) Option {
	return func(m *Masker) { m.conf = c }
}

// WithMinScore 覆盖采纳阈值。
func WithMinScore(s float32) Option {
	return func(m *Masker) { m.minScore = s }
}

func New(opts ...Option) *Masker {
	m := &Masker{minScore: DefaultMinScore}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Mask 把文本中的敏感区间替换成占位符，同时产出还原凭据。
// 被配置关闭的实体类型原样保留，也不写进凭据。
func (m *Masker) Mask(ctx context.Context, text string) (string, *types.MaskMeta, error) {
	return m.MaskWithLanguage(ctx, text, types.LangUnknown)
}

// MaskWithLanguage 指定语言脱敏。调用方明确知道语言时直接传，省掉一次判定，
// 也避免短文本被判错；lang 传空则自动判定。
func (m *Masker) MaskWithLanguage(ctx context.Context, text string, lang types.Language) (string, *types.MaskMeta, error) {
	meta := &types.MaskMeta{Original: text}
	if text == "" {
		return "", meta, nil
	}

	spans, err := m.detectSpans(ctx, text, lang)
	if err != nil {
		return "", nil, err
	}

	cfg := m.maskConfig()
	var b strings.Builder
	b.Grow(len(text))

	pos := 0
	var serial uint32
	for _, sp := range spans {
		if sp.Start > pos {
			b.WriteString(text[pos:sp.Start])
		}
		if cfg.Enabled(sp.Type) {
			serial++
			b.WriteString(placeholder.Render(sp.Type.String(), serial))
			meta.Items = append(meta.Items, types.MaskedItem{
				ID:    serial,
				Type:  sp.Type,
				Start: sp.Start,
				End:   sp.End,
				Score: sp.Score,
			})
		} else {
			b.WriteString(text[sp.Start:sp.End])
		}
		pos = sp.End
	}
	if pos < len(text) {
		b.WriteString(text[pos:])
	}
	return b.String(), meta, nil
}

// Spans 跑完整条脱敏流程后丢掉脱敏文本，只返回被记录下来的区间，用于排查与调参。
// 因此它同样受脱敏开关影响——被关掉的类型不会出现在结果里。
func (m *Masker) Spans(ctx context.Context, text string) ([]types.MaskedItem, error) {
	_, meta, err := m.MaskWithLanguage(ctx, text, types.LangUnknown)
	if err != nil {
		return nil, err
	}
	return meta.Items, nil
}

// MaskBatch 按输入顺序逐条脱敏，每条各自持有独立的还原凭据。
func (m *Masker) MaskBatch(ctx context.Context, texts []string) ([]string, []*types.MaskMeta, error) {
	masked := make([]string, len(texts))
	metas := make([]*types.MaskMeta, len(texts))
	for i, t := range texts {
		mt, meta, err := m.MaskWithLanguage(ctx, t, types.LangUnknown)
		if err != nil {
			return nil, nil, fmt.Errorf("mask text[%d]: %w", i, err)
		}
		masked[i], metas[i] = mt, meta
	}
	return masked, metas, nil
}

// detectSpans 汇总各识别器的结果并规整成互不重叠、按位置升序的区间。
func (m *Masker) detectSpans(ctx context.Context, text string, lang types.Language) ([]types.Span, error) {
	if lang == types.LangUnknown || lang == "auto" {
		lang = m.detect(text)
	}

	var merged []types.Span
	for _, r := range m.recognizers {
		spans, err := r.Recognize(ctx, text, lang)
		if err != nil {
			// 任一识别器失败即整体失败：宁可不发，也不能把没脱敏干净的文本送出去
			return nil, fmt.Errorf("recognizer %s: %w", r.Name(), err)
		}
		merged = append(merged, spans...)
	}

	if lang.IsChinese() {
		merged = mergeChineseAddress(text, merged)
	}

	merged = m.sanitize(merged, len(text))
	return span.Normalize(merged, m.minScore), nil
}

// mergeChineseAddress 用中文地址规则重组区间。
//
// NER 常把一个完整地址切成几段，融合后的地址区间才是准的，因此原有的地址区间
// 全部让位给融合结果；机构名若已被某个融合地址完整覆盖（例如「K11購物藝術館」
// 本身就是地址的一部分），也一并让位，避免同一段文字被两种类型重复认领。
func mergeChineseAddress(text string, spans []types.Span) []types.Span {
	seeds := make([]zhaddr.Seed, 0, len(spans))
	for _, sp := range spans {
		seeds = append(seeds, zhaddr.Seed{
			Kind:  seedKindOf(sp.Type),
			Start: sp.Start,
			End:   sp.End,
			Score: sp.Score,
		})
	}
	addrs := zhaddr.Merge(text, seeds)

	out := make([]types.Span, 0, len(spans)+len(addrs))
	for _, sp := range spans {
		if sp.Type == types.EntityPhysicalAddress {
			// 融合结果盖到了这一段就让位给它——融合后的边界更准。
			//
			// 但融合有可能什么都产不出来（层级链够不到隐私阈值，比如种子正好
			// 止于「科兴科学园」这类园区名，门牌号在规整时被判为层级倒挂删掉）。
			// 这种时候必须把原始区间留下：宁可按 NER 的边界脱敏，
			// 也不能让一整条已经识别出来的地址原样漏出去。
			if overlapsAny(sp, addrs) {
				continue
			}
			// 融合没产出时再看一眼种子本身：只到区县这种粒度定位不到人，
			// 按原有的隐私阈值丢弃；已经含门牌号的则保留。
			if !zhaddr.ContainsPrivateDetail(text, sp.Start, sp.End) {
				continue
			}
			out = append(out, sp)
			continue
		}
		if sp.Type == types.EntityOrganization && coveredByAny(sp, addrs) {
			continue
		}
		out = append(out, sp)
	}
	for _, a := range addrs {
		out = append(out, types.Span{
			Type:   types.EntityPhysicalAddress,
			Start:  a.Start,
			End:    a.End,
			Score:  a.Score,
			Source: "zhaddr",
		})
	}
	return out
}

func seedKindOf(t types.EntityType) zhaddr.SeedKind {
	switch t {
	case types.EntityPhysicalAddress:
		return zhaddr.SeedAddress
	case types.EntityOrganization:
		return zhaddr.SeedOrganization
	default:
		return zhaddr.SeedOther
	}
}

// overlapsAny 判断区间是否与任一融合结果相交。
// 这里用「相交」而不是「被完全覆盖」：融合结果与原始种子只要有重叠，
// 就说明它们指的是同一处地址，留两份会让后续的重叠消解按分数二选一，
// 反而可能丢掉融合补全出来的部分。
func overlapsAny(sp types.Span, addrs []zhaddr.Result) bool {
	for _, a := range addrs {
		if sp.Start < a.End && a.Start < sp.End {
			return true
		}
	}
	return false
}

func coveredByAny(sp types.Span, addrs []zhaddr.Result) bool {
	for _, a := range addrs {
		if a.Start <= sp.Start && a.End >= sp.End {
			return true
		}
	}
	return false
}

// sanitize 丢弃越界或空的区间，避免识别器的坐标错误直接损坏输出文本。
func (m *Masker) sanitize(spans []types.Span, textLen int) []types.Span {
	out := spans[:0]
	for _, sp := range spans {
		if !sp.Type.Valid() || !sp.ValidIn(textLen) {
			continue
		}
		out = append(out, sp)
	}
	return out
}

func (m *Masker) detect(text string) types.Language {
	if m.detector == nil {
		return types.LangUnknown
	}
	return m.detector.Detect(text)
}

func (m *Masker) maskConfig() types.MaskConfig {
	if m.conf == nil {
		return types.DefaultMaskConfig()
	}
	return m.conf.Mask()
}
