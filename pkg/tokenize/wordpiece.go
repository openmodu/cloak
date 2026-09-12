package tokenize

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Tokenizer 是 BERT 的两段式分词：先 BasicTokenizer 切成词，再 WordPiece 切成子词。
type Tokenizer struct {
	vocab *Vocab
	cfg   Config
	strip transform.Transformer
}

// Encoding 是一次编码结果。Tokens 与 IDs 一一对应，且已含首尾的 [CLS]/[SEP]。
type Encoding struct {
	IDs      []int64
	Tokens   []string
	Specials []bool // 与 Tokens 对齐，标记该位置是否是特殊 token
}

func New(vocab *Vocab, cfg Config) *Tokenizer {
	t := &Tokenizer{vocab: vocab, cfg: cfg}
	// strip_accents 未显式指定时，跟随 do_lower_case——与 HF 的默认行为一致
	stripAccents := cfg.DoLowerCase
	if cfg.StripAccents != nil {
		stripAccents = *cfg.StripAccents
	}
	if stripAccents {
		t.strip = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	}
	return t
}

// Load 从模型目录加载词表与配置并构造分词器。
func Load(modelDir string) (*Tokenizer, error) {
	vocab, err := LoadVocab(modelDir)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(modelDir)
	if err != nil {
		return nil, err
	}
	return New(vocab, cfg), nil
}

// Encode 把文本编码成模型输入。maxLen <= 0 表示不截断。
func (t *Tokenizer) Encode(text string, maxLen int) Encoding {
	var enc Encoding
	t.appendSpecial(&enc, t.cfg.ClsToken)

	for _, word := range t.basicTokenize(text) {
		for _, piece := range t.wordPiece(word) {
			id, ok := t.vocab.ID(piece)
			if !ok {
				id, _ = t.vocab.ID(t.cfg.UnkToken)
				piece = t.cfg.UnkToken
			}
			enc.IDs = append(enc.IDs, int64(id))
			enc.Tokens = append(enc.Tokens, piece)
			enc.Specials = append(enc.Specials, false)
		}
	}

	// 截断时给 [SEP] 留一个位置
	if maxLen > 0 && len(enc.IDs)+1 > maxLen {
		keep := maxLen - 1
		enc.IDs = enc.IDs[:keep]
		enc.Tokens = enc.Tokens[:keep]
		enc.Specials = enc.Specials[:keep]
	}
	t.appendSpecial(&enc, t.cfg.SepToken)
	return enc
}

func (t *Tokenizer) appendSpecial(enc *Encoding, tok string) {
	id, ok := t.vocab.ID(tok)
	if !ok {
		return
	}
	enc.IDs = append(enc.IDs, int64(id))
	enc.Tokens = append(enc.Tokens, tok)
	enc.Specials = append(enc.Specials, true)
}

// basicTokenize 按空白与标点切词，并把每个 CJK 字符单独成词——这是 BERT 处理中文的方式。
func (t *Tokenizer) basicTokenize(text string) []string {
	var out []string
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}

	for _, r := range text {
		switch {
		case r == 0 || r == 0xFFFD || isControl(r):
			continue
		case unicode.IsSpace(r):
			flush()
		case isCJK(r):
			flush()
			out = append(out, string(r))
		case isPunct(r):
			flush()
			out = append(out, string(r))
		default:
			cur.WriteRune(t.normalizeRune(r))
		}
	}
	flush()

	if t.strip != nil {
		for i, w := range out {
			if s, _, err := transform.String(t.strip, w); err == nil {
				out[i] = s
			}
		}
	}
	return out
}

func (t *Tokenizer) normalizeRune(r rune) rune {
	if t.cfg.DoLowerCase {
		return unicode.ToLower(r)
	}
	return r
}

// wordPiece 是贪心最长匹配：首个子词原样查表，后续子词加 "##" 前缀。
func (t *Tokenizer) wordPiece(word string) []string {
	const maxChars = 100
	if word == "" {
		return nil
	}
	runesOf := []rune(word)
	if len(runesOf) > maxChars {
		return []string{t.cfg.UnkToken}
	}

	var pieces []string
	start := 0
	for start < len(runesOf) {
		end := len(runesOf)
		var found string
		for start < end {
			sub := string(runesOf[start:end])
			if start > 0 {
				sub = "##" + sub
			}
			if _, ok := t.vocab.ID(sub); ok {
				found = sub
				break
			}
			end--
		}
		if found == "" {
			// 整词无法切分，按 HF 的做法整体记为 [UNK]
			return []string{t.cfg.UnkToken}
		}
		pieces = append(pieces, found)
		start = end
	}
	return pieces
}

func isControl(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return false
	}
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}

// isPunct 与 HF 的 _is_punctuation 一致：ASCII 区的符号也算标点，不只是 Unicode P 类。
func isPunct(r rune) bool {
	if (r >= '!' && r <= '/') || (r >= ':' && r <= '@') ||
		(r >= '[' && r <= '`') || (r >= '{' && r <= '~') {
		return true
	}
	return unicode.IsPunct(r)
}

// isCJK 覆盖 HF _is_chinese_char 的同一组区间。
func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF,
		r >= 0x3400 && r <= 0x4DBF,
		r >= 0x20000 && r <= 0x2A6DF,
		r >= 0x2A700 && r <= 0x2B73F,
		r >= 0x2B740 && r <= 0x2B81F,
		r >= 0x2B820 && r <= 0x2CEAF,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0x2F800 && r <= 0x2FA1F:
		return true
	}
	return false
}
