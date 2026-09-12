package tokenize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsupportedTokenizer 表示模型声明的分词流水线超出本包的实现范围。
//
// 这类不匹配不会报错，只会悄悄产出错位的 token 与偏移，最后表现为
// 「模型好像不太准」。与其如此，不如在加载阶段就明确失败。
type ErrUnsupportedTokenizer struct {
	Reason string
}

func (e *ErrUnsupportedTokenizer) Error() string {
	return "tokenize: 不支持的分词器配置: " + e.Reason
}

// tokenizerJSON 只取本包关心的几段。
type tokenizerJSON struct {
	Model struct {
		Type string `json:"type"`
	} `json:"model"`
	Normalizer   *normalizerJSON `json:"normalizer"`
	PreTokenizer *componentJSON  `json:"pre_tokenizer"`
}

type normalizerJSON struct {
	Type               string            `json:"type"`
	Lowercase          *bool             `json:"lowercase"`
	StripAccents       *bool             `json:"strip_accents"`
	CleanText          *bool             `json:"clean_text"`
	HandleChineseChars *bool             `json:"handle_chinese_chars"`
	Normalizers        []*normalizerJSON `json:"normalizers"`
}

type componentJSON struct {
	Type          string           `json:"type"`
	PreTokenizers []*componentJSON `json:"pretokenizers"`
}

// inspectTokenizerJSON 读取模型目录里的 tokenizer.json，校验流水线并取出
// 归一化开关。文件不存在时一切照旧，由 tokenizer_config.json 决定行为。
func inspectTokenizerJSON(modelDir string, cfg *Config) error {
	b, err := os.ReadFile(filepath.Join(modelDir, "tokenizer.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var doc tokenizerJSON
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("tokenize: 解析 tokenizer.json 失败: %w", err)
	}

	if t := doc.Model.Type; t != "" && !strings.EqualFold(t, "WordPiece") {
		return &ErrUnsupportedTokenizer{Reason: "model.type = " + t + "，本包只实现了 WordPiece"}
	}
	if err := checkPreTokenizer(doc.PreTokenizer); err != nil {
		return err
	}
	return applyNormalizer(doc.Normalizer, cfg)
}

// checkPreTokenizer 只放行 BERT 那套按空白与标点切分的预分词。
func checkPreTokenizer(c *componentJSON) error {
	if c == nil || c.Type == "" {
		return nil
	}
	switch c.Type {
	case "BertPreTokenizer":
		return nil
	case "Sequence":
		if len(c.PreTokenizers) != 1 {
			return &ErrUnsupportedTokenizer{Reason: "pre-tokenizer Sequence must contain exactly one BertPreTokenizer"}
		}
		for _, sub := range c.PreTokenizers {
			if err := checkPreTokenizer(sub); err != nil {
				return err
			}
		}
		return nil
	default:
		return &ErrUnsupportedTokenizer{Reason: "pre_tokenizer = " + c.Type + "，本包只实现了 BERT 式预分词"}
	}
}

// applyNormalizer 校验归一化配置，并把 BertNormalizer 的开关写回 cfg。
// tokenizer.json 比 tokenizer_config.json 更贴近真实行为，因此优先级更高。
func applyNormalizer(n *normalizerJSON, cfg *Config) error {
	if n == nil || n.Type == "" {
		return nil
	}
	switch n.Type {
	case "BertNormalizer":
		if (n.CleanText != nil && !*n.CleanText) || (n.HandleChineseChars != nil && !*n.HandleChineseChars) {
			return &ErrUnsupportedTokenizer{Reason: "BERT clean_text and handle_chinese_chars must be enabled"}
		}
		cfg.DoLowerCase = true
		cfg.StripAccents = nil
		if n.Lowercase != nil {
			cfg.DoLowerCase = *n.Lowercase
		}
		if n.StripAccents != nil {
			v := *n.StripAccents
			cfg.StripAccents = &v
		}
		return nil
	case "Sequence":
		if len(n.Normalizers) != 1 {
			return &ErrUnsupportedTokenizer{Reason: "normalizer Sequence must contain exactly one BertNormalizer"}
		}
		for _, sub := range n.Normalizers {
			if err := applyNormalizer(sub, cfg); err != nil {
				return err
			}
		}
		return nil
	default:
		return &ErrUnsupportedTokenizer{Reason: "normalizer = " + n.Type + "，本包无法复现它的行为"}
	}
}
