// Package zhaddr 实现中文地址的成分识别与跨片段融合。
//
// NER 模型经常把一个完整地址切成好几段
// （「北京市朝阳区」「建国路88号」「国贸中心A座1208室」），本包按行政区划、
// 道路、楼栋、楼层、房间这条由粗到细的层级链，把这些碎片重新粘成完整地址，
// 并在扩展过程中顺着原文向左右延伸，补回 NER 漏掉的部分。
//
// 本包不认识任何业务概念：输入是带类别标记的种子区间，输出是地址区间，
// 全部按 UTF-8 字节偏移工作。
package zhaddr

import "sort"

// SeedKind 标记种子区间的来源类别。只有地址与机构名会被当作地址链的起点，
// 其余一律忽略。
type SeedKind uint8

const (
	SeedOther SeedKind = iota
	SeedAddress
	SeedOrganization
)

// Seed 是一段候选区间，通常来自 NER 的识别结果。
type Seed struct {
	Kind  SeedKind
	Start int
	End   int
	Score float32
}

// Result 是融合后的地址区间。Score 由地址链能到达的最细层级决定，层级越细分数越高。
type Result struct {
	Start int
	End   int
	Score float32
}

const (
	// scanWindow 是每次向外扩展时的扫描窗口（字节）。
	scanWindow = 96
	// maxTotalGrowChars 限制向右总共能长出多少个字符，防止跨句吞并。
	maxTotalGrowChars = 48
)

// Merge 把种子区间融合成完整地址。
//
// 未达到「隐私阈值」的链会被丢弃：必须出现门牌号（L5），或者同时出现 POI（L4）
// 与楼层/房间（L2/L1）。只到区县、道路这种粒度不足以定位到人，不算敏感地址。
func Merge(text string, seeds []Seed) []Result {
	mergedAdjacent := mergeAdjacentSeeds(text, seeds)
	consumed := make([]bool, len(mergedAdjacent))
	var out []Result

	for idx, sp := range mergedAdjacent {
		if consumed[idx] {
			continue
		}
		if sp.Kind != SeedAddress && sp.Kind != SeedOrganization {
			continue
		}

		newStart, newEnd := sp.Start, sp.End
		bits, _, tokenEnd := tokenizeWindow(text, sp.Start, sp.End, newEnd)
		newEnd = tokenEnd

		newStart, newEnd, bits = extendRight(text, sp.End, newStart, newEnd, bits)
		newStart, bits = extendLeft(text, newStart, bits)

		if !reachedPrivacyThreshold(bits) {
			continue
		}

		// 分数随最细层级递增：score = 0.9999 - lowest_level * 0.0025
		lowest := lowestRankInBits(bits)
		res := Result{
			Start: newStart,
			End:   newEnd,
			Score: float32(0.9999 - float64(lowest)*0.0025),
		}
		out = append(out, res)

		// 后续被本次结果完全覆盖的种子不再单独处理
		for j := idx + 1; j < len(mergedAdjacent); j++ {
			spj := mergedAdjacent[j]
			if spj.Start >= res.Start && spj.End <= res.End &&
				(spj.Kind == SeedAddress || spj.Kind == SeedOrganization) {
				consumed[j] = true
			}
		}
	}
	return out
}

// ContainsPrivateDetail 判断一段文本里是否出现过门牌号，或者 POI 加楼层/房间——
// 也就是够不够细到能定位到具体住户。
//
// 它只看窗口里**出现过什么成分**，不做层级链规整。规整是为了把碎片拼成一条完整
// 地址用的，会把顺序倒挂的成分删掉；而判断「这段文字敏不敏感」不该受拼接顺序影响。
// 调用方在融合产不出结果时用它兜底，避免把已经含门牌号的地址整条放过。
func ContainsPrivateDetail(text string, start, end int) bool {
	if start < 0 || end > len(text) || start >= end {
		return false
	}
	bits, _, _ := scanTokens(text, start, end, end)
	return reachedPrivacyThreshold(bits)
}

// reachedPrivacyThreshold 判断地址链是否细到足以定位到具体住户。
func reachedPrivacyThreshold(bits uint32) bool {
	return bits&bitL5 != 0 ||
		(bits&bitL4 != 0 && (bits&bitL2 != 0 || bits&bitL1 != 0))
}

// mergeAdjacentSeeds 先把仅隔着轻分隔符的相邻地址种子合成一段，
// 同时滤掉既不是地址也不是机构名的种子。
func mergeAdjacentSeeds(text string, seeds []Seed) []Seed {
	if len(seeds) == 0 {
		return nil
	}
	tmp := make([]Seed, len(seeds))
	copy(tmp, seeds)
	sort.SliceStable(tmp, func(i, j int) bool {
		if tmp[i].Start == tmp[j].Start {
			return tmp[i].End > tmp[j].End
		}
		return tmp[i].Start < tmp[j].Start
	})

	var out []Seed
	cur := tmp[0]
	for i := 1; i < len(tmp); i++ {
		nxt := tmp[i]
		if cur.Kind != SeedAddress && cur.Kind != SeedOrganization {
			cur = nxt
			continue
		}
		if cur.Kind == SeedAddress && nxt.Kind == SeedAddress && nxt.Start >= cur.End &&
			onlyLight(text, cur.End, nxt.Start) {
			cur.End = nxt.End
			if nxt.Score > cur.Score {
				cur.Score = nxt.Score
			}
			continue
		}
		out = append(out, cur)
		cur = nxt
	}
	if cur.Kind == SeedAddress || cur.Kind == SeedOrganization {
		out = append(out, cur)
	}
	return out
}

func onlyLight(text string, from, to int) bool {
	for p := from; p < to; p++ {
		if !isASCIILight(text[p]) {
			return false
		}
	}
	return true
}

// extendRight 沿原文向右扩展地址链，每轮取窗口内第一个能接上的成分。
func extendRight(text string, seedEnd, newStart, newEnd int, bits uint32) (int, int, uint32) {
	for {
		if countCharsBetween(text, seedEnd, newEnd) >= maxTotalGrowChars {
			break
		}
		// 已经到达隐私阈值后，不再跨过重分隔符进入下一个子句
		hasPriv := bits&(bitL5|bitL4|bitL3|bitL2|bitL1) != 0
		if hasPriv && heavySepAt(text, newEnd) > 0 {
			break
		}

		winEnd := newEnd + scanWindow
		if winEnd > len(text) {
			winEnd = len(text)
		}
		if winEnd <= newEnd {
			break
		}
		_, tokens, _ := tokenizeWindow(text, newEnd, winEnd, winEnd)
		if len(tokens) == 0 {
			break
		}

		accepted := false
		for _, tk := range tokens {
			// L8 互斥：已经有区县了就不再接一个
			if tk.level == L8District && bits&bitL8 != 0 {
				continue
			}
			// 已达隐私阈值又遇到更粗的行政层级，说明下一个地址开始了，停在这里
			if hasPriv && levelRank(tk.level) >= levelRank(L8District) {
				accepted = false
				break
			}
			newBits := canRightAttach(bits, tk.level, text, newStart, newEnd, tk.start, tk.end)
			if newBits == 0 {
				continue
			}
			bits = newBits
			if tk.start < newStart {
				newStart = tk.start
			}
			newEnd = tk.end
			accepted = true
			break
		}
		if !accepted {
			break
		}
	}
	return newStart, newEnd, bits
}

// extendLeft 沿原文向左扩展，每轮取窗口内最靠近起点、且能接上的成分。
func extendLeft(text string, newStart int, bits uint32) (int, uint32) {
	for {
		winStart := newStart - scanWindow
		if winStart < 0 {
			winStart = 0
		}
		if winStart == newStart {
			break
		}
		_, tokens, _ := tokenizeWindow(text, winStart, newStart, newStart)
		if len(tokens) == 0 {
			break
		}

		accepted := false
		for j := len(tokens) - 1; j >= 0; j-- {
			tk := tokens[j]
			if tk.level == L8District && bits&bitL8 != 0 {
				continue
			}
			newBits := canLeftAttach(bits, tk.level, text, newStart, tk.start, tk.end)
			if newBits == 0 {
				continue
			}
			bits = newBits
			newStart = tk.start
			accepted = true
			break
		}
		if !accepted {
			break
		}
	}
	return newStart, bits
}
