package types

// MaskedItem 记录一处被替换掉的敏感内容：占位符编号、类型，以及它在原文中的位置。
// 刻意不存原文副本，还原时按位置从 Original 切片取。
type MaskedItem struct {
	ID    uint32     `json:"id"`
	Type  EntityType `json:"type"`
	Start int        `json:"start"`
	End   int        `json:"end"`
	Score float32    `json:"score"`
}

// MaskMeta 是一次脱敏的还原凭据。脱敏与还原可能跨请求甚至跨进程，
// 因此它必须是可序列化的自包含结构。
type MaskMeta struct {
	Original string       `json:"original"`
	Items    []MaskedItem `json:"items"`
}

// Lookup 按占位符编号取回对应的原文片段。
func (m *MaskMeta) Lookup(id uint32) (string, bool) {
	if m == nil {
		return "", false
	}
	for _, it := range m.Items {
		if it.ID != id {
			continue
		}
		sp := Span{Type: it.Type, Start: it.Start, End: it.End}
		if !sp.ValidIn(len(m.Original)) {
			return "", false
		}
		return m.Original[it.Start:it.End], true
	}
	return "", false
}
