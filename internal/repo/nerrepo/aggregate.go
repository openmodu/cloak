package nerrepo

import (
	"strings"

	"github.com/openmodu/cloak/internal/types"
)

// aggregate 把 BIO 标注的 token 聚合成完整实体区间，
// 是 core/NerRecognizer.zig 里 run + aggregateNerRecogEntityToRecogEntity 的移植。
//
// 规则：遇到 Begin 开一个实体，后续 Inside 且类型相同就并进来，分数取滑动平均；
// 遇到类型变化、遇到 None、或遇到新的 Begin 就收尾。
func aggregate(text string, tokens []types.NERToken, source string) []types.Span {
	var out []types.Span
	idx := 0
	for idx < len(tokens) {
		sp, next := aggregateOne(text, tokens, idx)
		idx = next
		if sp.Type != types.EntityNone {
			sp.Source = source
			out = append(out, sp)
		}
	}
	return out
}

// aggregateOne 从 from 开始聚合一个实体，返回该实体与下一个起点。
func aggregateOne(text string, tokens []types.NERToken, from int) (types.Span, int) {
	var (
		span      types.Span
		haveEntry bool
	)
	i := from
	for ; i < len(tokens); i++ {
		tok := tokens[i]
		isBegin := tok.Tag == types.TagBegin

		if tok.Type == types.EntityNone {
			if haveEntry {
				break
			}
			continue
		}

		if !haveEntry {
			if !isBegin {
				continue // 没有 Begin 打头的 Inside 片段直接丢掉
			}
			haveEntry = true
			span = types.Span{Type: tok.Type, Start: tok.Start, End: tok.End, Score: tok.Score}
			continue
		}

		if tok.Type != span.Type {
			break // 换了类型，收尾
		}

		score := (span.Score + tok.Score) / 2
		switch {
		case !isBegin:
			span.End = tok.End
			span.Score = score
		case hasSubwordPrefix(sliceOf(text, tok.Start, tok.End)):
			// 这里检查的是**原文切片**是否以 "##" 开头，而不是 token 串——
			// 兜底用于原文本身就含 "##" 的场景，实际极少触发。
			span.End = tok.End
			span.Score = score
		default:
			// 同类型的新 Begin，说明是下一个实体
			return finish(span, haveEntry), i
		}
	}
	return finish(span, haveEntry), i
}

func finish(span types.Span, have bool) types.Span {
	if !have {
		return types.Span{}
	}
	return span
}

func hasSubwordPrefix(word string) bool { return strings.HasPrefix(word, "##") }

func sliceOf(text string, start, end int) string {
	if start < 0 || end > len(text) || start >= end {
		return ""
	}
	return text[start:end]
}
