// Package tokenize 提供 BERT 系模型的 WordPiece 分词，与业务无关。
//
// 本包只负责产出**正确的 token 串与 id**，不负责偏移：
// token 在原文中的位置由 internal/repo/nerrepo/offsets.go 拿 token 串回原文重新定位。
// 这样分词器换实现时，偏移逻辑不受影响。
package tokenize

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Vocab 是词表：token 到 id 的映射。
type Vocab struct {
	ids    map[string]int32
	tokens []string
}

func (v *Vocab) Size() int { return len(v.tokens) }

func (v *Vocab) ID(token string) (int32, bool) {
	id, ok := v.ids[token]
	return id, ok
}

func (v *Vocab) Token(id int32) (string, bool) {
	if id < 0 || int(id) >= len(v.tokens) {
		return "", false
	}
	return v.tokens[id], true
}

// LoadVocab 从模型目录加载词表，优先 vocab.txt，其次 tokenizer.json 里的 model.vocab。
func LoadVocab(modelDir string) (*Vocab, error) {
	if v, err := loadVocabTxt(filepath.Join(modelDir, "vocab.txt")); err == nil {
		return v, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return loadTokenizerJSON(filepath.Join(modelDir, "tokenizer.json"))
}

func loadVocabTxt(path string) (*Vocab, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	v := &Vocab{ids: map[string]int32{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		tok := strings.TrimRight(sc.Text(), "\n\r")
		v.ids[tok] = int32(len(v.tokens))
		v.tokens = append(v.tokens, tok)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(v.tokens) == 0 {
		return nil, fmt.Errorf("vocab %s 是空的", path)
	}
	return v, nil
}

func loadTokenizerJSON(path string) (*Vocab, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Model struct {
			Vocab map[string]int32 `json:"vocab"`
		} `json:"model"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Model.Vocab) == 0 {
		return nil, fmt.Errorf("%s 里没有 model.vocab", path)
	}
	v := &Vocab{ids: doc.Model.Vocab, tokens: make([]string, len(doc.Model.Vocab))}
	for tok, id := range doc.Model.Vocab {
		if int(id) >= len(v.tokens) {
			grown := make([]string, id+1)
			copy(grown, v.tokens)
			v.tokens = grown
		}
		v.tokens[id] = tok
	}
	return v, nil
}

// Config 是分词行为开关，取自模型目录下的 tokenizer_config.json。
type Config struct {
	DoLowerCase  bool
	StripAccents *bool
	UnkToken     string
	ClsToken     string
	SepToken     string
}

// DefaultConfig 是 BERT uncased 的常见取值。
func DefaultConfig() Config {
	return Config{DoLowerCase: true, UnkToken: "[UNK]", ClsToken: "[CLS]", SepToken: "[SEP]"}
}

// LoadConfig 读取 tokenizer_config.json，缺失字段回落到默认值。
func LoadConfig(modelDir string) (Config, error) {
	cfg := DefaultConfig()
	b, err := os.ReadFile(filepath.Join(modelDir, "tokenizer_config.json"))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var doc struct {
		DoLowerCase          *bool    `json:"do_lower_case"`
		DoBasicTokenize      *bool    `json:"do_basic_tokenize"`
		TokenizeChineseChars *bool    `json:"tokenize_chinese_chars"`
		NeverSplit           []string `json:"never_split"`
		StripAccents         *bool    `json:"strip_accents"`
		UnkToken             *string  `json:"unk_token"`
		ClsToken             *string  `json:"cls_token"`
		SepToken             *string  `json:"sep_token"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return cfg, fmt.Errorf("parse tokenizer_config.json: %w", err)
	}
	if doc.DoLowerCase != nil {
		cfg.DoLowerCase = *doc.DoLowerCase
	}
	if (doc.DoBasicTokenize != nil && !*doc.DoBasicTokenize) || (doc.TokenizeChineseChars != nil && !*doc.TokenizeChineseChars) || len(doc.NeverSplit) > 0 {
		return cfg, &ErrUnsupportedTokenizer{Reason: "disabled basic/Chinese tokenization or never_split is unsupported"}
	}
	cfg.StripAccents = doc.StripAccents
	if doc.UnkToken != nil {
		cfg.UnkToken = *doc.UnkToken
	}
	if doc.ClsToken != nil {
		cfg.ClsToken = *doc.ClsToken
	}
	if doc.SepToken != nil {
		cfg.SepToken = *doc.SepToken
	}
	return cfg, nil
}

// NewVocab 用一组按 id 顺序排列的 token 构造词表，便于测试与内嵌小词表。
func NewVocab(tokens []string) *Vocab {
	v := &Vocab{ids: make(map[string]int32, len(tokens)), tokens: tokens}
	for i, t := range tokens {
		v.ids[t] = int32(i)
	}
	return v
}
