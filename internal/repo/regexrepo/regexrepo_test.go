package regexrepo

import (
	"context"
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

const hex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// recognizeAll 跑完整套内置识别器，模拟真实装配。
func recognizeAll(t *testing.T, text string) []types.Span {
	t.Helper()
	set, err := NewDefaultSet()
	if err != nil {
		t.Fatal(err)
	}
	var out []types.Span
	for _, r := range set {
		spans, err := r.Recognize(context.Background(), text, types.LangEnglish)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, spans...)
	}
	return out
}

func has(spans []types.Span, text string, typ types.EntityType, want string) bool {
	for _, sp := range spans {
		if sp.Type == typ && sp.Text(text) == want {
			return true
		}
	}
	return false
}

// 识别器必须按实体枚举顺序建出，这个顺序会影响同分同范围区间的取舍。
func TestDefaultSetFollowsEntityOrder(t *testing.T) {
	set, err := NewDefaultSet()
	if err != nil {
		t.Fatal(err)
	}
	want := types.AllEntityTypes()
	if len(set) != len(want) {
		t.Fatalf("want %d recognizers, got %d", len(want), len(set))
	}
	for i, r := range set {
		if r.EntityType() != want[i] {
			t.Fatalf("第 %d 个识别器应为 %s，实际 %s", i, want[i], r.EntityType())
		}
	}
}

// 每个识别器只产出自己负责的那一种类型。
func TestRecognizerEmitsOwnTypeOnly(t *testing.T) {
	r, err := New(types.EntityEmailAddress, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := "mail a@b.io tel 4155552671 url https://x.io/a"
	spans, err := r.Recognize(context.Background(), text, types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].Type != types.EntityEmailAddress {
		t.Fatalf("unexpected spans: %+v", spans)
	}
}

func TestRecognizeCommonEntities(t *testing.T) {
	cases := []struct {
		name string
		text string
		typ  types.EntityType
		want string
	}{
		{"email", "contact a.b+1@test.io now", types.EntityEmailAddress, "a.b+1@test.io"},
		{"url", "see https://example.com/x?a=1 ok", types.EntityURLAddress, "https://example.com/x?a=1"},
		{"phone", "call +1 415-555-2671 now", types.EntityPhoneNumber, "+1 415-555-2671"},
		{"bank", "acct 4242424242424242 end", types.EntityBankNumber, "4242424242424242"},
		{"vcode", "code 123456 end", types.EntityVerificationCode, "123456"},
		{"hexkey", "key " + hex64 + " end", types.EntityPrivateKey, hex64},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if spans := recognizeAll(t, c.text); !has(spans, c.text, c.typ, c.want) {
				t.Fatalf("未识别出 %s=%q，实际: %+v", c.typ, c.want, spans)
			}
		})
	}
}

// 带标签的规则只应脱敏值本身，不能把标签一起换掉。
func TestGroupOnlyCapturesValue(t *testing.T) {
	text := "the pwd: S3cure!Passw0rd (reset later)"
	spans := recognizeAll(t, text)
	if !has(spans, text, types.EntityPassword, "S3cure!Passw0rd") {
		t.Fatalf("密码值未被单独捕获: %+v", spans)
	}
	for _, sp := range spans {
		if sp.Type == types.EntityPassword && sp.Text(text) != "S3cure!Passw0rd" {
			t.Fatalf("捕获范围过宽: %q", sp.Text(text))
		}
	}
}

func TestLabeledVerificationCode(t *testing.T) {
	text := "your verification code: 9F4T2A thanks"
	spans := recognizeAll(t, text)
	if !has(spans, text, types.EntityVerificationCode, "9F4T2A") {
		t.Fatalf("未捕获带标签验证码: %+v", spans)
	}
}

// 同一条规则要能在一段文本里扫出多个匹配。
func TestScanFindsAllMatches(t *testing.T) {
	text := "a@b.io and c@d.io and e@f.io"
	r, err := New(types.EntityEmailAddress, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	spans, err := r.Recognize(context.Background(), text, types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 3 {
		t.Fatalf("want 3 matches, got %d: %+v", len(spans), spans)
	}
}

// ValidateFunc 只调整分数，不否决匹配——要让匹配落选就返回一个低于阈值的分数。
func TestValidateOnlyAdjustsScore(t *testing.T) {
	r, err := New(types.EntityVerificationCode, nil, func(s string) (float32, bool) {
		if s == "1234" {
			return 0.1, true // 分数压到阈值以下，后续规整时会被丢掉
		}
		return 0, false // 不表态，沿用规则自带分数
	})
	if err != nil {
		t.Fatal(err)
	}
	spans, err := r.Recognize(context.Background(), "1234 and 5678", types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]float32 = map[string]float32{}
	for _, sp := range spans {
		got[sp.Text("1234 and 5678")] = sp.Score
	}
	if got["1234"] != 0.1 {
		t.Fatalf("校验函数给出的分数未生效: %+v", got)
	}
	if got["5678"] != 0.5 {
		t.Fatalf("未表态时应沿用规则默认分数: %+v", got)
	}
}

// 内置规则已包含的表达式不会因为再传一次而重复扫描。
func TestExtraSpecDedup(t *testing.T) {
	dup := PresetSpecsFor(types.EntityEmailAddress)[0]
	r, err := New(types.EntityEmailAddress, []PatternSpec{dup}, nil)
	if err != nil {
		t.Fatal(err)
	}
	spans, err := r.Recognize(context.Background(), "a@b.io", types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
}

func TestBadPatternIsRejected(t *testing.T) {
	if _, err := New(types.EntityEmailAddress, []PatternSpec{{Name: "bad", Pattern: "("}}, nil); err == nil {
		t.Fatal("非法正则应当在构造时报错")
	}
	if _, err := New(types.EntityEmailAddress, []PatternSpec{{Name: "grp", Pattern: `\d+`, GroupIndex: 2}}, nil); err == nil {
		t.Fatal("越界的捕获组应当在构造时报错")
	}
}
