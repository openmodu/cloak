// Package nerrepo 用本地 NER 模型识别敏感实体，实现 usecase.Recognizer。
//
// 推理本身交给 Inferencer 接口，本包负责推理前后的全部处理——分词、softmax、
// token 回原文定位、相邻片段合并、BIO 聚合、标签到实体类型的映射。
// 这样换推理后端（本地 ONNX、远程服务）不影响这里的任何逻辑。
//
// 模型不可用时返回 ErrModelUnavailable，由装配层决定是否接入——
// 缺模型只该退化成「只有正则」，不该让整个服务起不来。
package nerrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/pkg/tokenize"
)

// ErrModelUnavailable 表示模型目录不存在或缺少必要文件。
var ErrModelUnavailable = errors.New("nerrepo: NER 模型不可用")

// Inferencer 是模型推理的抽象：输入 token id 序列，输出每个位置在各标签上的 logits。
// ONNX Runtime 是其中一种实现，换成远程推理服务也只需替换这一层。
type Inferencer interface {
	Infer(ctx context.Context, inputIDs, attentionMask, tokenTypeIDs []int64) ([][]float32, error)
	Close() error
}

// TextConverter 用于简繁转换：繁体模型配简体输入时，理想做法是
// 简→繁 喂模型、繁→简 还原 token，好让 token 能在原文里定位到。
// 不需要转换时传 nil 即可。
type TextConverter interface {
	ToModel(text string) string   // 喂给模型前的转换
	ToSource(token string) string // token 回原文定位前的反向转换
}

// MaxSeqLen 是截断长度，与常见 BERT 系模型的位置编码上限一致。
const MaxSeqLen = 512

type Recognizer struct {
	name      string
	tokenizer *tokenize.Tokenizer
	inf       Inferencer
	id2label  map[int]string
	converter TextConverter
	ignore    map[string]bool
	forLang   func(types.Language) bool
}

type Option func(*Recognizer)

// WithConverter 接入简繁转换。
func WithConverter(c TextConverter) Option {
	return func(r *Recognizer) { r.converter = c }
}

// WithLanguageFilter 限定该识别器只在特定语言下参与识别。
// 中英文各挂一个模型时用它做分流，不匹配的语言直接跳过，省掉一次推理。
func WithLanguageFilter(f func(types.Language) bool) Option {
	return func(r *Recognizer) { r.forLang = f }
}

// WithIgnoreLabels 覆盖要忽略的标签，默认只忽略 "O"。
func WithIgnoreLabels(labels ...string) Option {
	return func(r *Recognizer) {
		r.ignore = map[string]bool{}
		for _, l := range labels {
			r.ignore[l] = true
		}
	}
}

// New 用已经准备好的分词器与推理器构造识别器。
func New(name string, tk *tokenize.Tokenizer, inf Inferencer, id2label map[int]string, opts ...Option) *Recognizer {
	r := &Recognizer{
		name:      name,
		tokenizer: tk,
		inf:       inf,
		id2label:  id2label,
		ignore:    map[string]bool{"O": true},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// LoadFromDir 按约定的目录布局加载模型：
//
//	<modelDir>/vocab.txt 或 tokenizer.json
//	<modelDir>/config.json          （提供 id2label）
//	<modelDir>/onnx/model_quantized.onnx
//
// 推理器由调用方传入，因为它的实现取决于编译时是否带上 ONNX Runtime。
func LoadFromDir(name, modelDir string, inf Inferencer, opts ...Option) (*Recognizer, error) {
	if st, err := os.Stat(modelDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrModelUnavailable, modelDir)
	}
	tk, err := tokenize.Load(modelDir)
	if err != nil {
		return nil, fmt.Errorf("%w: 加载分词器失败: %v", ErrModelUnavailable, err)
	}
	id2label, err := LoadID2Label(modelDir)
	if err != nil {
		return nil, err
	}
	return New(name, tk, inf, id2label, opts...), nil
}

// LoadID2Label 从 config.json 读取标签表。没有标签表就无法把输出映射成实体类型，
// 因此这里直接报错，而不是退化成 LABEL_0 这种没有意义的输出。
func LoadID2Label(modelDir string) (map[int]string, error) {
	b, err := os.ReadFile(filepath.Join(modelDir, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("%w: 读取 config.json 失败: %v", ErrModelUnavailable, err)
	}
	var doc struct {
		ID2Label map[string]string `json:"id2label"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%w: 解析 config.json 失败: %v", ErrModelUnavailable, err)
	}
	if len(doc.ID2Label) == 0 {
		return nil, fmt.Errorf("%w: config.json 里没有 id2label", ErrModelUnavailable)
	}
	out := make(map[int]string, len(doc.ID2Label))
	for k, v := range doc.ID2Label {
		id, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		out[id] = v
	}
	return out, nil
}

func (r *Recognizer) Name() string { return r.name }

func (r *Recognizer) Close() error {
	if r.inf == nil {
		return nil
	}
	return r.inf.Close()
}

// runOptions 是一次识别的可选参数。
type runOptions struct {
	offsetText     string
	tokenTransform func(string) string
	ignore         map[string]bool
}

func (o runOptions) isIgnored(label string) bool { return o.ignore[label] }

// Recognize 跑一次完整的 NER 流程。
func (r *Recognizer) Recognize(ctx context.Context, text string, lang types.Language) ([]types.Span, error) {
	if text == "" || r.inf == nil || r.tokenizer == nil {
		return nil, nil
	}
	if r.forLang != nil && !r.forLang(lang) {
		return nil, nil
	}

	// 简繁不匹配时：转换后的文本喂模型，偏移仍然按原文算
	runText := text
	opts := runOptions{offsetText: text, ignore: r.ignore}
	if r.converter != nil {
		runText = r.converter.ToModel(text)
		opts.tokenTransform = r.converter.ToSource
	}

	enc := r.tokenizer.Encode(runText, MaxSeqLen)
	attention := make([]int64, len(enc.IDs))
	tokenTypes := make([]int64, len(enc.IDs))
	for i := range attention {
		attention[i] = 1
	}

	logits, err := r.inf.Infer(ctx, enc.IDs, attention, tokenTypes)
	if err != nil {
		return nil, fmt.Errorf("nerrepo: 推理失败: %w", err)
	}

	items := classify(enc, logits, r.id2label, opts)
	return aggregate(text, toNERTokens(items), r.name), nil
}
