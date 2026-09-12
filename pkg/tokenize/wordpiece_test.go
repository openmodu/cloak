package tokenize

import (
	"strings"
	"testing"
)

func testTokenizer(t *testing.T, extra ...string) *Tokenizer {
	t.Helper()
	vocab := append([]string{
		"[PAD]", "[UNK]", "[CLS]", "[SEP]",
		"john", "doe", "acme", "corp", "##oration", "at", "lives", "##n",
		"北", "京", "市", "朝", "阳", "区", "，", ".",
	}, extra...)
	return New(NewVocab(vocab), DefaultConfig())
}

func tokensOnly(enc Encoding) []string {
	out := make([]string, 0, len(enc.Tokens))
	for i, tok := range enc.Tokens {
		if enc.Specials[i] {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func TestEncodeAddsSpecialTokens(t *testing.T) {
	enc := testTokenizer(t).Encode("john", 0)
	if len(enc.Tokens) != 3 || enc.Tokens[0] != "[CLS]" || enc.Tokens[2] != "[SEP]" {
		t.Fatalf("unexpected tokens: %v", enc.Tokens)
	}
	if !enc.Specials[0] || enc.Specials[1] || !enc.Specials[2] {
		t.Fatalf("unexpected specials: %v", enc.Specials)
	}
}

func TestWordPieceSplitsSubwords(t *testing.T) {
	got := tokensOnly(testTokenizer(t).Encode("Acme Corporation", 0))
	want := []string{"acme", "corp", "##oration"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// 中文按字切分，这是 BERT 处理 CJK 的方式。
func TestCJKSplitsPerCharacter(t *testing.T) {
	got := tokensOnly(testTokenizer(t).Encode("北京市朝阳区", 0))
	want := []string{"北", "京", "市", "朝", "阳", "区"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnknownWordBecomesUNK(t *testing.T) {
	got := tokensOnly(testTokenizer(t).Encode("zzzz", 0))
	if len(got) != 1 || got[0] != "[UNK]" {
		t.Fatalf("got %v", got)
	}
}

func TestPunctuationIsSeparate(t *testing.T) {
	got := tokensOnly(testTokenizer(t).Encode("john.", 0))
	want := []string{"john", "."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// 超长输入要截断，并且始终给 [SEP] 留位置。
func TestTruncationKeepsSep(t *testing.T) {
	enc := testTokenizer(t).Encode("北京市朝阳区", 4)
	if len(enc.Tokens) != 4 {
		t.Fatalf("want 4 tokens, got %d: %v", len(enc.Tokens), enc.Tokens)
	}
	if enc.Tokens[len(enc.Tokens)-1] != "[SEP]" {
		t.Fatalf("末位应当是 [SEP]: %v", enc.Tokens)
	}
}
