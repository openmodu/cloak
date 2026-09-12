package span

import "testing"

type iv struct {
	s, e int
	w    float32
	tag  string
}

func (i iv) Bounds() (int, int) { return i.s, i.e }
func (i iv) Weight() float32    { return i.w }

func tags(xs []iv) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.tag)
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNormalizeKeepsHighestScoreOnOverlap(t *testing.T) {
	// 长跨度高分区间应当吃掉内部的低分碎片，模拟 PHONE 覆盖多个 VCODE 的场景
	in := []iv{
		{s: 10, e: 14, w: 0.5, tag: "vcode-a"},
		{s: 0, e: 19, w: 0.7, tag: "phone"},
		{s: 15, e: 19, w: 0.5, tag: "vcode-b"},
		{s: 30, e: 40, w: 0.9, tag: "email"},
	}
	eq(t, tags(Normalize(in, 0.5)), []string{"phone", "email"})
}

func TestNormalizeDropsBelowThreshold(t *testing.T) {
	in := []iv{
		{s: 0, e: 5, w: 0.49, tag: "low"},
		{s: 6, e: 9, w: 0.5, tag: "edge"}, // 等于阈值应当保留
	}
	eq(t, tags(Normalize(in, 0.5)), []string{"edge"})
}

func TestDedupSameRangeKeepsHigher(t *testing.T) {
	in := []iv{
		{s: 0, e: 5, w: 0.6, tag: "low"},
		{s: 0, e: 5, w: 0.8, tag: "high"},
	}
	SortByStart(in)
	eq(t, tags(DedupSameRange(in)), []string{"high"})
}

func TestResolveOverlapPrefersLongerOnTie(t *testing.T) {
	// 同分时更长的优先，避免把一个完整实体切碎
	in := []iv{
		{s: 0, e: 4, w: 0.7, tag: "short"},
		{s: 0, e: 9, w: 0.7, tag: "long"},
	}
	eq(t, tags(ResolveOverlap(in)), []string{"long"})
}

func TestResolveOverlapAdjacentIsNotOverlap(t *testing.T) {
	// [0,5) 与 [5,9) 相邻但不相交，两个都该保留
	in := []iv{
		{s: 0, e: 5, w: 0.7, tag: "a"},
		{s: 5, e: 9, w: 0.7, tag: "b"},
	}
	eq(t, tags(ResolveOverlap(in)), []string{"a", "b"})
}

func TestNormalizeEmpty(t *testing.T) {
	if got := Normalize([]iv(nil), 0.5); len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}
