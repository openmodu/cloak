package masker

import (
	"context"
	"strings"
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

// zhDetector 固定返回简体中文，用来触发中文地址融合。
type zhDetector struct{}

func (zhDetector) Detect(string) types.Language { return types.LangZhHans }

// 中文地址融合的价值就在这里：NER 只圈出「北京市朝阳区」，
// 融合后要顺着原文长成完整地址，把门牌号、楼栋、房间都包进来。
func TestChineseAddressIsExtendedFromNERSeed(t *testing.T) {
	const text = "我的家庭住址是：北京市朝阳区建国路88号国贸中心A座1208室（对，就是上次那个）。"
	const seed = "北京市朝阳区"
	const want = "北京市朝阳区建国路88号国贸中心A座1208室"

	start := strings.Index(text, seed)
	m := New(
		WithRecognizers(stubRecognizer{name: "ner", spans: []types.Span{
			{Type: types.EntityPhysicalAddress, Start: start, End: start + len(seed), Score: 0.9},
		}}),
		WithLangDetector(zhDetector{}),
		WithConfigStore(&stubConf{cfg: types.EnableAllMaskConfig()}),
	)

	items, err := m.Spans(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %+v", items)
	}
	if got := text[items[0].Start:items[0].End]; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// 只到区县这种粒度定位不到具体住户，达不到隐私阈值，不该被当成敏感地址。
func TestCoarseChineseAddressIsNotMasked(t *testing.T) {
	const text = "我在上海市浦东新区上班。"
	const seed = "上海市浦东新区"

	start := strings.Index(text, seed)
	m := New(
		WithRecognizers(stubRecognizer{name: "ner", spans: []types.Span{
			{Type: types.EntityPhysicalAddress, Start: start, End: start + len(seed), Score: 0.9},
		}}),
		WithLangDetector(zhDetector{}),
		WithConfigStore(&stubConf{cfg: types.EnableAllMaskConfig()}),
	)

	items, err := m.Spans(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("want none, got %+v", items)
	}
}

// 英文文本不走中文规则，地址区间原样保留。
func TestNonChineseSkipsAddressMerge(t *testing.T) {
	const text = "北京市朝阳区建国路88号国贸中心A座1208室"
	const seed = "北京市朝阳区"

	start := strings.Index(text, seed)
	m := New(
		WithRecognizers(stubRecognizer{name: "ner", spans: []types.Span{
			{Type: types.EntityPhysicalAddress, Start: start, End: start + len(seed), Score: 0.9},
		}}),
		WithConfigStore(&stubConf{cfg: types.EnableAllMaskConfig()}),
		// 不给 detector，语言是未知，不触发中文规则
	)

	items, err := m.Spans(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %+v", items)
	}
	if got := text[items[0].Start:items[0].End]; got != seed {
		t.Fatalf("got %q, want %q", got, seed)
	}
}

// 被融合地址完整覆盖的机构名要让位，避免同一段文字被两种类型重复认领。
func TestOrganizationCoveredByAddressIsDropped(t *testing.T) {
	const text = "九龍尖沙咀彌敦道128號K11購物藝術館6樓"
	const org = "K11購物藝術館"

	orgStart := strings.Index(text, org)
	m := New(
		WithRecognizers(stubRecognizer{name: "ner", spans: []types.Span{
			{Type: types.EntityPhysicalAddress, Start: 0, End: len("九龍尖沙咀彌敦道128號"), Score: 0.9},
			{Type: types.EntityOrganization, Start: orgStart, End: orgStart + len(org), Score: 0.9},
		}}),
		WithLangDetector(zhDetector{}),
		WithConfigStore(&stubConf{cfg: types.EnableAllMaskConfig()}),
	)

	items, err := m.Spans(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Type == types.EntityOrganization {
			t.Fatalf("机构名应当让位给融合后的地址: %+v", items)
		}
	}
	if len(items) != 1 || text[items[0].Start:items[0].End] != text {
		t.Fatalf("融合结果应当覆盖整段地址: %+v", items)
	}
}
