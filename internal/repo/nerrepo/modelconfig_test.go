package nerrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordingInferencer 只记下收到的序列长度，用来验证截断真的按模型配置生效。
type recordingInferencer struct {
	gotLen   int
	numLabel int
}

func (r *recordingInferencer) Infer(_ context.Context, ids, _, _ []int64) ([][]float32, error) {
	r.gotLen = len(ids)
	out := make([][]float32, len(ids))
	for i := range out {
		row := make([]float32, r.numLabel)
		row[0] = 5 // 全标成 O，本测试只关心长度
		out[i] = row
	}
	return out, nil
}

func (r *recordingInferencer) Close() error { return nil }

// writeModelDir 造一个最小可加载的模型目录。
func writeModelDir(t *testing.T, configJSON string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	vocab := strings.Join(append([]string{}, testVocab...), "\n")
	if err := os.WriteFile(filepath.Join(dir, "vocab.txt"), []byte(vocab), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const labelsJSON = `"id2label":{"0":"O","1":"B-PER","2":"I-PER"}`

func TestLoadModelConfigReadsMaxPositionEmbeddings(t *testing.T) {
	dir := writeModelDir(t, `{`+labelsJSON+`,"max_position_embeddings":128}`)
	cfg, err := LoadModelConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxSeqLen != 128 {
		t.Fatalf("want 128, got %d", cfg.MaxSeqLen)
	}
	if cfg.ID2Label[1] != "B-PER" {
		t.Fatalf("标签表没读对: %+v", cfg.ID2Label)
	}
}

// 缺字段或取值不合法时退回默认值，而不是让模型加载失败。
func TestLoadModelConfigFallsBackToDefault(t *testing.T) {
	for name, body := range map[string]string{
		"字段缺失": `{` + labelsJSON + `}`,
		"取值为零": `{` + labelsJSON + `,"max_position_embeddings":0}`,
		"取值为负": `{` + labelsJSON + `,"max_position_embeddings":-1}`,
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadModelConfig(writeModelDir(t, body))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.MaxSeqLen != DefaultMaxSeqLen {
				t.Fatalf("want %d, got %d", DefaultMaxSeqLen, cfg.MaxSeqLen)
			}
		})
	}
}

// 没有标签表就无法把输出映射成实体类型，必须报错。
func TestLoadModelConfigRequiresLabels(t *testing.T) {
	if _, err := LoadModelConfig(writeModelDir(t, `{"max_position_embeddings":128}`)); err == nil {
		t.Fatal("want error")
	}
}

// 关键一条：模型自报的上限要真的传到分词截断上。
// 送超过位置编码上限的序列会让推理直接失败或产出垃圾，所以这里必须截住。
func TestMaxSeqLenFromConfigIsApplied(t *testing.T) {
	dir := writeModelDir(t, `{`+labelsJSON+`,"max_position_embeddings":8}`)
	inf := &recordingInferencer{numLabel: 3}
	r, err := LoadFromDir("ner:test", dir, inf)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.TrimSpace(strings.Repeat("北京市朝阳区 ", 20))
	if _, err := r.Recognize(context.Background(), long, "zh-Hans"); err != nil {
		t.Fatal(err)
	}
	if inf.gotLen != 8 {
		t.Fatalf("应当截断到 8 个 token，实际送了 %d 个", inf.gotLen)
	}
}

// 调用方显式指定时覆盖模型配置。
func TestWithMaxSeqLenOverridesConfig(t *testing.T) {
	dir := writeModelDir(t, `{`+labelsJSON+`,"max_position_embeddings":128}`)
	inf := &recordingInferencer{numLabel: 3}
	r, err := LoadFromDir("ner:test", dir, inf, WithMaxSeqLen(6))
	if err != nil {
		t.Fatal(err)
	}
	long := strings.TrimSpace(strings.Repeat("北京市朝阳区 ", 20))
	if _, err := r.Recognize(context.Background(), long, "zh-Hans"); err != nil {
		t.Fatal(err)
	}
	if inf.gotLen != 6 {
		t.Fatalf("want 6, got %d", inf.gotLen)
	}
}

// 直接用 New 构造（不读目录）时保持默认值。
func TestNewUsesDefaultMaxSeqLen(t *testing.T) {
	r := newTestRecognizer(t, nil)
	if r.maxSeqLen != DefaultMaxSeqLen {
		t.Fatalf("want %d, got %d", DefaultMaxSeqLen, r.maxSeqLen)
	}
}
