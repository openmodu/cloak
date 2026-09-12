package nerrepo

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// 本文件负责把 token 重新定位回原文。
//
// 这里刻意不用分词器返回的 offset mapping，而是拿 token 字符串回原文里按游标查找，
// 查不到再退回「去掉重音与连接符」后再查。原因是分词器的 offset 行为各家不一致，
// 而脱敏的切分位置完全由它决定——自己算一遍，换分词器时结果才稳定。
//
// 一处必要的调整：Python 那边按码点索引计算，最后再转成字节偏移；Go 的字符串本身
// 就是字节序列，这里全程按字节走，并且把大小写转换的索引映射显式建出来——
// 否则像 'İ'、'K' 这种转换后字节长度会变的字符会让偏移整体错位。

// lowerWithMap 返回小写化文本，以及「小写文本字节索引 → 原文字节索引」的映射。
// 映射长度为 len(lower)+1，末位指向 len(text)，方便直接映射区间右端。
func lowerWithMap(text string) (string, []int) {
	var b strings.Builder
	b.Grow(len(text))
	mapping := make([]int, 0, len(text)+1)

	for i, r := range text {
		lr := unicode.ToLower(r)
		n := b.Len()
		b.WriteRune(lr)
		for j := n; j < b.Len(); j++ {
			mapping = append(mapping, i)
		}
	}
	mapping = append(mapping, len(text))
	return b.String(), mapping
}

var accentStripper = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// stripAccents 去掉组合重音符号。
func stripAccents(s string) string {
	out, _, err := transform.String(accentStripper, s)
	if err != nil {
		return s
	}
	return out
}

// isConnectorPunct 匹配各种连字符、撇号、间隔点。
// 这些字符在人名里常见（O'Brien、让-皮埃尔），分词器往往会吃掉它们，
// 去掉之后更容易和 token 对上。
func isConnectorPunct(r rune) bool {
	switch r {
	case '-', '\'', '`', '−', '·', '・', '⁃', '∙':
		return true
	}
	return r >= '‐' && r <= '―'
}

// buildStrippedMap 返回「去掉重音与连接符」后的文本，以及每个字节到原文字节索引的映射。
func buildStrippedMap(s string) (string, []int) {
	var b strings.Builder
	b.Grow(len(s))
	mapping := make([]int, 0, len(s))

	for i, r := range s {
		for _, c := range norm.NFD.String(string(r)) {
			if unicode.Is(unicode.Mn, c) {
				continue
			}
			if isConnectorPunct(c) {
				continue
			}
			n := b.Len()
			b.WriteRune(c)
			for j := n; j < b.Len(); j++ {
				mapping = append(mapping, i)
			}
		}
	}
	return b.String(), mapping
}

// findStrippedIndexAtOrAfter 二分查出第一个映射值不小于 origIndex 的位置。
func findStrippedIndexAtOrAfter(mapping []int, origIndex int) int {
	return sort.Search(len(mapping), func(i int) bool { return mapping[i] >= origIndex })
}

// computeOffsetsFromTokens 把 token 序列逐个定位回文本，返回每个 token 的字节区间。
// 定位不到的 token 退化成零宽区间，停在当前游标处：宁可少标一个，
// 也不能让后续 token 的偏移整体错位。
func computeOffsetsFromTokens(text string, tokens []string) [][2]int {
	offsets := make([][2]int, len(tokens))
	lowerText, lowerToOrig := lowerWithMap(text)
	stripped, strippedToOrig := buildStrippedMap(lowerText)

	cursorLower, cursorStripped := 0, 0
	for i, full := range tokens {
		raw := strings.TrimPrefix(full, "##")
		if raw == "" {
			pos := origIndex(lowerToOrig, cursorLower, len(text))
			offsets[i] = [2]int{pos, pos}
			continue
		}
		tokLower := strings.ToLower(raw)

		if p := indexFrom(lowerText, tokLower, cursorLower); p >= 0 {
			e := p + len(tokLower)
			offsets[i] = [2]int{origIndex(lowerToOrig, p, len(text)), origIndex(lowerToOrig, e, len(text))}
			cursorLower = e
			cursorStripped = findStrippedIndexAtOrAfter(strippedToOrig, e)
			continue
		}

		// 退一步：去掉重音与连接符再找，处理「O'Brien」「José」这类
		tokStripped := stripAccents(tokLower)
		if tokStripped != "" {
			if sp := indexFrom(stripped, tokStripped, cursorStripped); sp >= 0 {
				after := sp + len(tokStripped)
				startOrig := lookup(strippedToOrig, sp, len(lowerText))
				endOrig := lookup(strippedToOrig, after, len(lowerText))
				offsets[i] = [2]int{
					origIndex(lowerToOrig, startOrig, len(text)),
					origIndex(lowerToOrig, endOrig, len(text)),
				}
				cursorLower = endOrig
				cursorStripped = after
				continue
			}
		}

		pos := origIndex(lowerToOrig, cursorLower, len(text))
		offsets[i] = [2]int{pos, pos}
	}
	return offsets
}

// indexFrom 先从游标处找，找不到再从头找一遍。
// 回头找是为了容忍分词器与原文顺序不完全一致的情况。
func indexFrom(hay, needle string, from int) int {
	if from < 0 {
		from = 0
	}
	if from <= len(hay) {
		if p := strings.Index(hay[from:], needle); p >= 0 {
			return from + p
		}
	}
	return strings.Index(hay, needle)
}

func origIndex(mapping []int, i, fallback int) int {
	if i < 0 {
		return 0
	}
	if i < len(mapping) {
		return mapping[i]
	}
	return fallback
}

func lookup(mapping []int, i, fallback int) int {
	if i < 0 {
		return 0
	}
	if i < len(mapping) {
		return mapping[i]
	}
	return fallback
}
