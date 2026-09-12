package nerrepo

import (
	"context"
	"testing"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/pkg/tokenize"
)

// fakeInferencer 按 token 串给标签，替代真实模型，
// 让整条 NER 流水线（分词 → softmax → 定位 → 合并 → BIO 聚合）可以脱离模型文件测试。
type fakeInferencer struct {
	tokens   []string
	labelOf  map[string]int
	numLabel int
}

func (f *fakeInferencer) Infer(_ context.Context, ids, _, _ []int64) ([][]float32, error) {
	out := make([][]float32, len(ids))
	for i, id := range ids {
		row := make([]float32, f.numLabel)
		for j := range row {
			row[j] = -5
		}
		tok := f.tokens[int(id)]
		idx, ok := f.labelOf[tok]
		if !ok {
			idx = 0 // O
		}
		row[idx] = 5
		out[i] = row
	}
	return out, nil
}

func (f *fakeInferencer) Close() error { return nil }

var testVocab = []string{
	"[PAD]", "[UNK]", "[CLS]", "[SEP]",
	"john", "doe", "acme", "corp", "##oration", "works", "at",
	"北", "京", "市", "朝", "阳", "区",
}

var testLabels = map[int]string{
	0: "O", 1: "B-PER", 2: "I-PER", 3: "B-ORG", 4: "I-ORG", 5: "B-LOC", 6: "I-LOC",
}

func newTestRecognizer(t *testing.T, labelOf map[string]int, opts ...Option) *Recognizer {
	t.Helper()
	tk := tokenize.New(tokenize.NewVocab(testVocab), tokenize.DefaultConfig())
	inf := &fakeInferencer{tokens: testVocab, labelOf: labelOf, numLabel: len(testLabels)}
	return New("ner:test", tk, inf, testLabels, opts...)
}

func TestRecognizeEndToEnd(t *testing.T) {
	r := newTestRecognizer(t, map[string]int{
		"john": 1, "doe": 2, // B-PER, I-PER
		"acme": 3, "corp": 4, "##oration": 4, // B-ORG, I-ORG
	})
	text := "John Doe works at Acme Corporation"
	spans, err := r.Recognize(context.Background(), text, types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 2 {
		t.Fatalf("want 2 spans, got %+v", spans)
	}
	if got := spans[0].Text(text); got != "John Doe" {
		t.Fatalf("人名区间错了: %q", got)
	}
	if spans[0].Type != types.EntityUserName {
		t.Fatalf("人名类型错了: %v", spans[0].Type)
	}
	if got := spans[1].Text(text); got != "Acme Corporation" {
		t.Fatalf("机构区间错了: %q", got)
	}
	if spans[1].Type != types.EntityOrganization {
		t.Fatalf("机构类型错了: %v", spans[1].Type)
	}
}

// 中文按字标注，聚合后要还原成完整地名，且偏移落在字符边界上。
func TestRecognizeChinese(t *testing.T) {
	r := newTestRecognizer(t, map[string]int{
		"北": 5, "京": 6, "市": 6, "朝": 6, "阳": 6, "区": 6,
	})
	text := "北京市朝阳区"
	spans, err := r.Recognize(context.Background(), text, types.LangZhHans)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].Text(text) != "北京市朝阳区" {
		t.Fatalf("got %+v", spans)
	}
	if spans[0].Type != types.EntityPhysicalAddress {
		t.Fatalf("类型错了: %v", spans[0].Type)
	}
}

// 语言过滤器让中英两个模型各管各的，与上游按语言二选一等价。
func TestLanguageFilterSkipsRecognizer(t *testing.T) {
	r := newTestRecognizer(t,
		map[string]int{"john": 1},
		WithLanguageFilter(func(l types.Language) bool { return l.IsChinese() }),
	)
	spans, err := r.Recognize(context.Background(), "John", types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 0 {
		t.Fatalf("英文文本不该走中文模型: %+v", spans)
	}
}

func TestRecognizeEmptyText(t *testing.T) {
	r := newTestRecognizer(t, nil)
	spans, err := r.Recognize(context.Background(), "", types.LangEnglish)
	if err != nil || len(spans) != 0 {
		t.Fatalf("got %+v %v", spans, err)
	}
}

// 模型目录不存在时要给出明确的错误，让装配层能据此退化成纯正则。
func TestLoadFromDirMissing(t *testing.T) {
	if _, err := LoadFromDir("ner:x", "/definitely/not/here", nil); err == nil {
		t.Fatal("want error")
	}
}
