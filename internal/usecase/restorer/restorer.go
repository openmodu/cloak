// Package restorer 实现还原用例：把大模型返回文本里的占位符换回原始敏感内容。
package restorer

import (
	"context"
	"sort"
	"strings"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/pkg/placeholder"
)

// Restorer 是还原用例对象。还原是纯函数式的，不依赖任何外部能力。
type Restorer struct{}

func New() *Restorer { return &Restorer{} }

// slot 是一处待回填：masked 文本里占位符的位置，以及要填回去的原文。
type slot struct {
	start int
	end   int
	text  string
}

// Restore 由凭据驱动：为每条记录重新生成占位符文本，在 masked 里找它
// **第一次出现**的位置，按位置排序后一次性重建文本。
//
// 由此带来的行为同样保留：同一个占位符在回复里出现多次时只还原第一处；
// 凭据里有而回复里没有的（被模型吞掉）跳过；回复里出现的陌生占位符原样保留。
func (r *Restorer) Restore(_ context.Context, masked string, meta *types.MaskMeta) (string, error) {
	if meta == nil || len(meta.Items) == 0 || masked == "" {
		return masked, nil
	}

	slots := make([]slot, 0, len(meta.Items))
	for _, it := range meta.Items {
		ph := placeholder.Render(it.Type.String(), it.ID)
		start := strings.Index(masked, ph)
		if start < 0 {
			continue // 模型把这个占位符弄丢了，跳过即可，不该让整次还原失败
		}
		sp := types.Span{Type: it.Type, Start: it.Start, End: it.End}
		if !sp.ValidIn(len(meta.Original)) {
			continue
		}
		slots = append(slots, slot{start: start, end: start + len(ph), text: meta.Original[it.Start:it.End]})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].start < slots[j].start })

	var b strings.Builder
	b.Grow(len(masked) + len(meta.Original))
	pos := 0
	for _, s := range slots {
		if s.start > pos {
			b.WriteString(masked[pos:s.start])
		}
		b.WriteString(s.text)
		pos = s.end
	}
	if pos < len(masked) {
		b.WriteString(masked[pos:])
	}
	return b.String(), nil
}

// RestoreBatch 按输入顺序逐条还原，metas 与 texts 一一对应。
func (r *Restorer) RestoreBatch(ctx context.Context, masked []string, metas []*types.MaskMeta) ([]string, error) {
	out := make([]string, len(masked))
	for i, t := range masked {
		var meta *types.MaskMeta
		if i < len(metas) {
			meta = metas[i]
		}
		s, err := r.Restore(ctx, t, meta)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}
