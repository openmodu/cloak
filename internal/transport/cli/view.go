package cli

import "github.com/openmodu/cloak/internal/types"

// spanView 是 spans 子命令的输出结构，把内部类型翻译成可读 JSON。
type spanView struct {
	ID    uint32  `json:"id"`
	Type  string  `json:"type"`
	Text  string  `json:"text"`
	Start int     `json:"start"`
	End   int     `json:"end"`
	Score float32 `json:"score"`
}

func spansView(text string, items []types.MaskedItem) []spanView {
	out := make([]spanView, 0, len(items))
	for _, it := range items {
		sp := types.Span{Type: it.Type, Start: it.Start, End: it.End}
		out = append(out, spanView{
			ID:    it.ID,
			Type:  it.Type.String(),
			Text:  sp.Text(text),
			Start: it.Start,
			End:   it.End,
			Score: it.Score,
		})
	}
	return out
}
