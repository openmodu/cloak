package nerrepo

import (
	"math"
	"strings"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/pkg/tokenize"
)

// item 是 token 级别的分类结果，对应上游 libner.py 的 NerItem。
type item struct {
	entity string
	score  float32
	index  int
	word   string
	start  int
	end    int
}

// classify 把模型输出的 logits 转成 token 级结果，逐段对应上游
// TokenClassificationPipelinePy.run：
// 取 argmax 与 softmax 分数 → 跳过 O 与特殊 token → 用 token 串回原文定位 →
// 合并相邻的同类片段。
func classify(enc tokenize.Encoding, logits [][]float32, id2label map[int]string, opts runOptions) []item {
	// 先滤掉特殊 token，得到用于定位的纯 token 序列
	plain := make([]string, 0, len(enc.Tokens))
	seqToPlain := make([]int, len(enc.Tokens))
	for i := range seqToPlain {
		seqToPlain[i] = -1
	}
	for j, tok := range enc.Tokens {
		if tok == "" || (strings.HasPrefix(tok, "[") && strings.HasSuffix(tok, "]")) {
			continue
		}
		seqToPlain[j] = len(plain)
		plain = append(plain, strings.TrimPrefix(tok, "##"))
	}
	if opts.tokenTransform != nil {
		for i, tok := range plain {
			if converted := opts.tokenTransform(tok); converted != "" {
				plain[i] = converted
			}
		}
	}

	offsetText := opts.offsetText
	offsets := computeOffsetsFromTokens(offsetText, plain)

	var raw []item
	for j := 0; j < len(logits) && j < len(seqToPlain); j++ {
		label, score := argmaxSoftmax(logits[j], id2label)
		if opts.isIgnored(label) {
			continue
		}
		plainIdx := seqToPlain[j]
		if plainIdx < 0 || plainIdx >= len(plain) {
			continue
		}
		off := [2]int{}
		if plainIdx < len(offsets) {
			off = offsets[plainIdx]
		}
		raw = append(raw, item{
			entity: label,
			score:  score,
			index:  j,
			word:   plain[plainIdx],
			start:  off[0],
			end:    off[1],
		})
	}
	return mergeContiguous(raw)
}

// argmaxSoftmax 取最高分标签及其 softmax 概率（数值稳定写法）。
func argmaxSoftmax(row []float32, id2label map[int]string) (string, float32) {
	if len(row) == 0 {
		return "", 0
	}
	maxIdx := 0
	maxVal := row[0]
	for i, v := range row {
		if v > maxVal {
			maxVal, maxIdx = v, i
		}
	}
	var sum float64
	for _, v := range row {
		sum += math.Exp(float64(v - maxVal))
	}
	var score float32
	if sum > 0 {
		score = float32(1 / sum) // exp(max-max)=1
	}
	label, ok := id2label[maxIdx]
	if !ok {
		label = "LABEL_" + itoa(maxIdx)
	}
	return label, score
}

// mergeContiguous 合并核心类别相同且首尾相接的片段，分数取加权平均。
func mergeContiguous(raw []item) []item {
	var out []item
	var cur *item
	count := 0

	coreOf := func(s string) string {
		if len(s) > 2 {
			switch s[:2] {
			case "B-", "I-", "E-", "S-":
				return s[2:]
			}
		}
		return s
	}

	for _, it := range raw {
		if cur == nil {
			c := it
			cur, count = &c, 1
			continue
		}
		if coreOf(cur.entity) == coreOf(it.entity) && cur.end == it.start {
			cur.word += it.word
			cur.end = it.end
			cur.score = (cur.score*float32(count) + it.score) / float32(count+1)
			count++
			continue
		}
		out = append(out, *cur)
		c := it
		cur, count = &c, 1
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// toNERTokens 把 token 级结果翻译成核心层要的结构。
// 上游在绑定层还要做一次「码点偏移 → 字节偏移」的换算，Go 全程按字节走，这一步不存在。
func toNERTokens(items []item) []types.NERToken {
	out := make([]types.NERToken, 0, len(items))
	for _, it := range items {
		core, tag := splitLabel(it.entity)
		out = append(out, types.NERToken{
			Type:  labelToEntityType(core),
			Tag:   tag,
			Score: it.score,
			Index: it.index,
			Start: it.start,
			End:   it.end,
			Word:  it.word,
		})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
