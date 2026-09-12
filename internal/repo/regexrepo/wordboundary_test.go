package regexrepo

import (
	"context"
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

// 上游用的 Rust regex-automata 把汉字当词字符，「A座1208室」里的 1208 两侧都不成
// 词边界，因此不该被 \b\d{4,8}\b 命中。Go 的 \b 只认 ASCII，若不做翻译就会误判。
func TestUnicodeWordBoundary(t *testing.T) {
	r, err := New(types.EntityVerificationCode, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"汉字包夹的数字不是独立词", "国贸中心A座1208室", nil},
		{"空格分隔的数字是独立词", "code 1208 here", []string{"1208"}},
		{"中文标点也算分隔符", "验证码：1208。", []string{"1208"}},
		{"下划线相连不算边界", "johndoe_1984", nil},
		{"串首串尾视作非词字符", "1208", []string{"1208"}},
		{"连续数字过长不匹配", "18744325579", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spans, err := r.Recognize(context.Background(), c.text, types.LangZhHans)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, sp := range spans {
				got = append(got, sp.Text(c.text))
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

// 带标志位与捕获组的规则，翻译后组号要正确顺延。
func TestTranslatePreservesGroup(t *testing.T) {
	r, err := New(types.EntityPassword, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := "the pwd: S3cure!Passw0rd end"
	spans, err := r.Recognize(context.Background(), text, types.LangEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || spans[0].Text(text) != "S3cure!Passw0rd" {
		t.Fatalf("unexpected spans: %+v", spans)
	}
}

func TestTranslateRejectsInnerBoundary(t *testing.T) {
	if _, err := New(types.EntityEmailAddress, []PatternSpec{
		{Name: "inner", Pattern: `foo\bbar`},
	}, nil); err == nil {
		t.Fatal("表达式中间的 \\b 应当在构造时报错")
	}
}

func TestTranslateLeavesPlainPatternUntouched(t *testing.T) {
	got, shift, err := translateWordBoundaries(`https?://\S+`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `https?://\S+` || shift != 0 {
		t.Fatalf("got %q shift=%d", got, shift)
	}
}
