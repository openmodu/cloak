// Package masker 实现脱敏用例：编排识别器、规整区间、产出占位符文本与还原凭据。
// 流程逐段对应上游 core/aifw_core.zig 的 MaskPipeline.run。
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

// DefaultMinScore 是区间被采纳的最低置信度，与上游写死的 0.5 一致。
const DefaultMinScore float32 = 0.5

// Masker 是脱敏用例对象。它只持有接口，不关心识别能力来自正则还是模型。
type Masker struct {
	recognizers []usecase.Recognizer
	detector    usecase.LangDetector
	conf        usecase.ConfigStore
	minScore    float32
}

type Option func(*Masker)

// WithRecognizers 追加识别器。顺序会影响同分同范围区间的取舍，需与上游保持一致。
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
// 被配置关闭的实体类型原样保留，也不写进凭据——与上游一致。
func (m *Masker) Mask(ctx context.Context, text string) (string, *types.MaskMeta, error) {
	return m.MaskWithLanguage(ctx, text, types.LangUnknown)
}

// MaskWithLanguage 指定语言脱敏。上游的 core 接口同样把语言作为入参，由绑定层
// 在调用方没给的时候自动判定；lang 传空即走自动判定。
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

// Spans 对应上游的 aifw_session_get_pii_spans：跑完整条脱敏流程后丢掉masked 文本，
// 只返回被记录下来的区间。因此它同样受脱敏开关影响——被关掉的类型不会出现在结果里。
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
	if lang == types.LangUnknown {
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

// mergeChineseAddress 用中文地址规则重组区间，对应上游 MaskPipeline.run 的 3.1 步。
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
