package masker

import (
	"context"
	"errors"
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

// stubRecognizer 让用例层的测试不依赖任何真实识别实现——这正是端口存在的意义。
type stubRecognizer struct {
	name  string
	spans []types.Span
	err   error
}

func (s stubRecognizer) Name() string { return s.name }

func (s stubRecognizer) Recognize(context.Context, string, types.Language) ([]types.Span, error) {
	return s.spans, s.err
}

type stubConf struct{ cfg types.MaskConfig }

func (s *stubConf) Mask() types.MaskConfig     { return s.cfg }
func (s *stubConf) SetMask(c types.MaskConfig) { s.cfg = c }

func TestMaskReplacesEnabledTypesOnly(t *testing.T) {
	const text = "mail a@b.io tel 123"
	m := New(
		WithRecognizers(stubRecognizer{name: "stub", spans: []types.Span{
			{Type: types.EntityEmailAddress, Start: 5, End: 11, Score: 0.9},
			{Type: types.EntityPhoneNumber, Start: 16, End: 19, Score: 0.9},
		}}),
		// 只开邮箱：电话应当原样保留，且不写进还原凭据
		WithConfigStore(&stubConf{cfg: types.DisableAllMaskConfig().Enable(types.EntityEmailAddress)}),
	)

	masked, meta, err := m.Mask(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if want := "mail __PII_EMAIL_ADDRESS_1__ tel 123"; masked != want {
		t.Fatalf("got %q, want %q", masked, want)
	}
	if len(meta.Items) != 1 || meta.Items[0].Type != types.EntityEmailAddress {
		t.Fatalf("unexpected meta: %+v", meta.Items)
	}
}

func TestMaskSerialIsSequentialByPosition(t *testing.T) {
	const text = "a@b.io then c@d.io"
	m := New(WithRecognizers(stubRecognizer{name: "stub", spans: []types.Span{
		// 故意乱序给入，编号必须按文本位置而不是给入顺序
		{Type: types.EntityEmailAddress, Start: 12, End: 18, Score: 0.9},
		{Type: types.EntityEmailAddress, Start: 0, End: 6, Score: 0.9},
	}}))

	masked, _, err := m.Mask(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if want := "__PII_EMAIL_ADDRESS_1__ then __PII_EMAIL_ADDRESS_2__"; masked != want {
		t.Fatalf("got %q, want %q", masked, want)
	}
}

func TestMaskDropsOutOfRangeSpans(t *testing.T) {
	// 识别器给出越界坐标时必须被丢弃，而不是把输出文本切坏
	const text = "short"
	m := New(WithRecognizers(stubRecognizer{name: "stub", spans: []types.Span{
		{Type: types.EntityEmailAddress, Start: 3, End: 99, Score: 0.9},
		{Type: types.EntityEmailAddress, Start: 4, End: 4, Score: 0.9},
	}}))

	masked, meta, err := m.Mask(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if masked != text || len(meta.Items) != 0 {
		t.Fatalf("got %q meta=%+v", masked, meta.Items)
	}
}

func TestMaskPropagatesRecognizerError(t *testing.T) {
	// 识别器失败必须整体失败：宁可不发，也不能把没脱敏干净的文本送出去
	wantErr := errors.New("boom")
	m := New(WithRecognizers(
		stubRecognizer{name: "ok", spans: []types.Span{{Type: types.EntityEmailAddress, Start: 0, End: 1, Score: 0.9}}},
		stubRecognizer{name: "bad", err: wantErr},
	))
	if _, _, err := m.Mask(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestMaskEmptyText(t *testing.T) {
	m := New()
	masked, meta, err := m.Mask(context.Background(), "")
	if err != nil || masked != "" || len(meta.Items) != 0 {
		t.Fatalf("got %q %+v %v", masked, meta, err)
	}
}

func TestSpansAreNormalized(t *testing.T) {
	m := New(WithRecognizers(stubRecognizer{name: "stub", spans: []types.Span{
		{Type: types.EntityVerificationCode, Start: 2, End: 6, Score: 0.5},
		{Type: types.EntityPhoneNumber, Start: 0, End: 10, Score: 0.7},
		{Type: types.EntityPassword, Start: 0, End: 4, Score: 0.3}, // 低于阈值
	}}))

	items, err := m.Spans(context.Background(), "0123456789abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Type != types.EntityPhoneNumber {
		t.Fatalf("unexpected items: %+v", items)
	}
}

// Spans 跑的是完整脱敏流程，因此被开关关掉的类型不会出现在结果里。
func TestSpansRespectMaskConfig(t *testing.T) {
	m := New(
		WithRecognizers(stubRecognizer{name: "stub", spans: []types.Span{
			{Type: types.EntityEmailAddress, Start: 0, End: 6, Score: 0.9},
			{Type: types.EntityPhoneNumber, Start: 7, End: 10, Score: 0.9},
		}}),
		WithConfigStore(&stubConf{cfg: types.DisableAllMaskConfig().Enable(types.EntityEmailAddress)}),
	)

	items, err := m.Spans(context.Background(), "a@b.io 123")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Type != types.EntityEmailAddress {
		t.Fatalf("unexpected items: %+v", items)
	}
}
