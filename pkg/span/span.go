// Package span 提供区间运算：排序、过滤、去重、重叠消解。
//
// 它不认识任何业务语义（不知道什么是 PII、什么是实体类型），只要类型实现了
// Interval 就能参与运算，因此放在 pkg 下作为基础模块。
package span

import "sort"

// Interval 是参与区间运算的最小约束。Bounds 返回 [start, end) 的字节偏移，
// Weight 是消解冲突时的优先级权重（通常是识别置信度）。
type Interval interface {
	Bounds() (start, end int)
	Weight() float32
}

// Overlaps 判断两个半开区间是否相交。
func Overlaps(aStart, aEnd, bStart, bEnd int) bool {
	return !(aEnd <= bStart || bEnd <= aStart)
}

// SortByStart 按 start 升序、end 升序原地排序。
func SortByStart[T Interval](xs []T) {
	sort.SliceStable(xs, func(i, j int) bool {
		is, ie := xs[i].Bounds()
		js, je := xs[j].Bounds()
		if is == js {
			return ie < je
		}
		return is < js
	})
}

// FilterByWeight 丢弃权重低于 min 的区间。
func FilterByWeight[T Interval](xs []T, min float32) []T {
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if x.Weight() < min {
			continue
		}
		out = append(out, x)
	}
	return out
}

// DedupSameRange 合并起止完全相同的区间，保留权重最高的那个。
// 输入必须已经过 SortByStart。
func DedupSameRange[T Interval](xs []T) []T {
	out := make([]T, 0, len(xs))
	for _, cur := range xs {
		if len(out) == 0 {
			out = append(out, cur)
			continue
		}
		last := out[len(out)-1]
		cs, ce := cur.Bounds()
		ls, le := last.Bounds()
		if cs == ls && ce == le {
			if cur.Weight() > last.Weight() {
				out[len(out)-1] = cur
			}
			continue
		}
		out = append(out, cur)
	}
	return out
}

// ResolveOverlap 消解相交区间：按「权重高 > 跨度长 > 起点早」的优先级贪心选取，
// 已选中区间相交的候选一律丢弃。返回结果按 start 升序。
func ResolveOverlap[T Interval](xs []T) []T {
	if len(xs) == 0 {
		return xs
	}
	cand := make([]T, len(xs))
	copy(cand, xs)
	sort.SliceStable(cand, func(i, j int) bool {
		wi, wj := cand[i].Weight(), cand[j].Weight()
		if wi != wj {
			return wi > wj
		}
		is, ie := cand[i].Bounds()
		js, je := cand[j].Bounds()
		if li, lj := ie-is, je-js; li != lj {
			return li > lj
		}
		return is < js
	})

	picked := make([]T, 0, len(cand))
	for _, c := range cand {
		cs, ce := c.Bounds()
		ok := true
		for _, p := range picked {
			ps, pe := p.Bounds()
			if Overlaps(cs, ce, ps, pe) {
				ok = false
				break
			}
		}
		if ok {
			picked = append(picked, c)
		}
	}
	SortByStart(picked)
	return picked
}

// Normalize 是一条完整的规整流水线：排序 → 按权重过滤 → 同范围去重 → 重叠消解。
// 输出按 start 升序且两两不相交，可直接用于文本替换。
func Normalize[T Interval](xs []T, minWeight float32) []T {
	SortByStart(xs)
	xs = FilterByWeight(xs, minWeight)
	xs = DedupSameRange(xs)
	return ResolveOverlap(xs)
}
