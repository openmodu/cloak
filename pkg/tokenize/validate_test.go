package tokenize

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func modelDir(t *testing.T, tokenizerJSON string) string {
	t.Helper()
	dir := t.TempDir()
	vocab := "[PAD]\n[UNK]\n[CLS]\n[SEP]\njohn\n"
	if err := os.WriteFile(filepath.Join(dir, "vocab.txt"), []byte(vocab), 0o644); err != nil {
		t.Fatal(err)
	}
	if tokenizerJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "tokenizer.json"), []byte(tokenizerJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// 没有 tokenizer.json 时照旧，由 tokenizer_config.json 决定行为。
func TestLoadWithoutTokenizerJSON(t *testing.T) {
	if _, err := Load(modelDir(t, "")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAcceptsWordPiecePipeline(t *testing.T) {
	dir := modelDir(t, `{
	  "model": {"type": "WordPiece"},
	  "normalizer": {"type": "BertNormalizer", "lowercase": false, "strip_accents": false},
	  "pre_tokenizer": {"type": "BertPreTokenizer"}
	}`)
	if _, err := Load(dir); err != nil {
		t.Fatal(err)
	}
}

// tokenizer.json 的归一化开关比 tokenizer_config.json 更贴近真实行为，应当优先。
func TestNormalizerOverridesConfig(t *testing.T) {
	cfg := DefaultConfig() // DoLowerCase 默认 true
	dir := modelDir(t, `{"model":{"type":"WordPiece"},"normalizer":{"type":"BertNormalizer","lowercase":false}}`)
	if err := inspectTokenizerJSON(dir, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.DoLowerCase {
		t.Fatal("应当被 tokenizer.json 覆盖为不转小写")
	}
}

// 关键：实现范围之外的流水线必须明确失败，而不是静默算错。
func TestLoadRejectsUnsupportedPipelines(t *testing.T) {
	for name, doc := range map[string]string{
		"NFKC":                       `{"normalizer":{"type":"NFKC"}}`,
		"WhitespaceSplit":            `{"pre_tokenizer":{"type":"WhitespaceSplit"}}`,
		"Whitespace":                 `{"pre_tokenizer":{"type":"Whitespace"}}`,
		"Punctuation":                `{"pre_tokenizer":{"type":"Punctuation"}}`,
		"disabled cleaning":          `{"normalizer":{"type":"BertNormalizer","clean_text":false}}`,
		"disabled Chinese splitting": `{"normalizer":{"type":"BertNormalizer","handle_chinese_chars":false}}`,
		"BPE 模型":                     `{"model":{"type":"BPE"}}`,
		"Unigram 模型":                 `{"model":{"type":"Unigram"}}`,
		"ByteLevel 预分词":              `{"model":{"type":"WordPiece"},"pre_tokenizer":{"type":"ByteLevel"}}`,
		"Metaspace 预分词":              `{"model":{"type":"WordPiece"},"pre_tokenizer":{"type":"Metaspace"}}`,
		"Precompiled 归一化":            `{"model":{"type":"WordPiece"},"normalizer":{"type":"Precompiled"}}`,
		"Sequence 里藏着不支持的":           `{"model":{"type":"WordPiece"},"pre_tokenizer":{"type":"Sequence","pretokenizers":[{"type":"BertPreTokenizer"},{"type":"ByteLevel"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(modelDir(t, doc))
			var unsupported *ErrUnsupportedTokenizer
			if !errors.As(err, &unsupported) {
				t.Fatalf("应当报不支持，实际 %v", err)
			}
		})
	}
}

func TestLoadRejectsBrokenTokenizerJSON(t *testing.T) {
	if _, err := Load(modelDir(t, `{not json`)); err == nil {
		t.Fatal("want error")
	}
}

func TestRejectUnsupportedLegacyConfig(t *testing.T) {
	for _, doc := range []string{`{"do_basic_tokenize":false}`, `{"tokenize_chinese_chars":false}`, `{"never_split":["hello-world"]}`} {
		dir := modelDir(t, "")
		if err := os.WriteFile(filepath.Join(dir, "tokenizer_config.json"), []byte(doc), 0600); err != nil {
			t.Fatal(err)
		}
		var unsupported *ErrUnsupportedTokenizer
		if _, err := Load(dir); !errors.As(err, &unsupported) {
			t.Fatalf("accepted %s: %v", doc, err)
		}
	}
}
