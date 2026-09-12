package nerrepo

import (
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

func tok(typ types.EntityType, tag types.BIOTag, start, end int, score float32) types.NERToken {
	return types.NERToken{Type: typ, Tag: tag, Start: start, End: end, Score: score}
}

func TestAggregateJoinsBIORun(t *testing.T) {
	text := "John A. Doe works here"
	tokens := []types.NERToken{
		tok(types.EntityUserName, types.TagBegin, 0, 4, 0.9),
		tok(types.EntityUserName, types.TagInside, 5, 7, 0.8),
		tok(types.EntityUserName, types.TagInside, 8, 11, 0.7),
		tok(types.EntityNone, types.TagNone, 12, 17, 0.99),
	}
	spans := aggregate(text, tokens, "ner")
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %+v", spans)
	}
	if spans[0].Text(text) != "John A. Doe" {
		t.Fatalf("got %q", spans[0].Text(text))
	}
	// 分数是逐个滑动平均：((0.9+0.8)/2 + 0.7)/2
	if spans[0].Score < 0.77 || spans[0].Score > 0.78 {
		t.Fatalf("unexpected score %v", spans[0].Score)
	}
}

// 同类型的新 Begin 说明是下一个实体，不能并进上一个。
func TestAggregateSplitsOnNewBegin(t *testing.T) {
	text := "John Mary"
	tokens := []types.NERToken{
		tok(types.EntityUserName, types.TagBegin, 0, 4, 0.9),
		tok(types.EntityUserName, types.TagBegin, 5, 9, 0.9),
	}
	spans := aggregate(text, tokens, "ner")
	if len(spans) != 2 {
		t.Fatalf("want 2 spans, got %+v", spans)
	}
	if spans[0].Text(text) != "John" || spans[1].Text(text) != "Mary" {
		t.Fatalf("got %q / %q", spans[0].Text(text), spans[1].Text(text))
	}
}

func TestAggregateSplitsOnTypeChange(t *testing.T) {
	text := "John Acme"
	tokens := []types.NERToken{
		tok(types.EntityUserName, types.TagBegin, 0, 4, 0.9),
		tok(types.EntityOrganization, types.TagInside, 5, 9, 0.9),
	}
	spans := aggregate(text, tokens, "ner")
	// 第二个是 Inside 又换了类型：上一个收尾，它自己因为没有 Begin 打头被丢弃
	if len(spans) != 1 || spans[0].Text(text) != "John" {
		t.Fatalf("got %+v", spans)
	}
}

// 没有 Begin 打头的 Inside 片段直接丢掉，不能凭空造出实体。
func TestAggregateDropsLeadingInside(t *testing.T) {
	text := "John"
	tokens := []types.NERToken{tok(types.EntityUserName, types.TagInside, 0, 4, 0.9)}
	if spans := aggregate(text, tokens, "ner"); len(spans) != 0 {
		t.Fatalf("want none, got %+v", spans)
	}
}

func TestAggregateEmpty(t *testing.T) {
	if spans := aggregate("", nil, "ner"); len(spans) != 0 {
		t.Fatalf("want none, got %+v", spans)
	}
}

func TestSplitLabel(t *testing.T) {
	cases := []struct {
		in   string
		core string
		tag  types.BIOTag
	}{
		{"B-PER", "PER", types.TagBegin},
		{"S-ORG", "ORG", types.TagBegin},
		{"I-LOC", "LOC", types.TagInside},
		{"E-LOC", "LOC", types.TagInside},
		{"MISC", "MISC", types.TagNone},
		{"", "MISC", types.TagNone},
	}
	for _, c := range cases {
		core, tag := splitLabel(c.in)
		if core != c.core || tag != c.tag {
			t.Fatalf("%q -> (%q,%v), want (%q,%v)", c.in, core, tag, c.core, c.tag)
		}
	}
}

func TestLabelToEntityType(t *testing.T) {
	cases := map[string]types.EntityType{
		"PER":     types.EntityUserName,
		"PERSON":  types.EntityUserName,
		"ORG":     types.EntityOrganization,
		"LOC":     types.EntityPhysicalAddress,
		"GPE":     types.EntityPhysicalAddress,
		"FAC":     types.EntityPhysicalAddress,
		"ADDRESS": types.EntityPhysicalAddress,
		"MISC":    types.EntityNone,
		"WHAT":    types.EntityNone,
	}
	for in, want := range cases {
		if got := labelToEntityType(in); got != want {
			t.Fatalf("%q -> %v, want %v", in, got, want)
		}
	}
}
