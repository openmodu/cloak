package types

// Span 是文本中一段被识别出的敏感区间。Start/End 是 UTF-8 字节偏移，半开区间 [Start, End)。
type Span struct {
	Type   EntityType
	Start  int
	End    int
	Score  float32
	Source string // 产出该 Span 的识别器名字，仅用于诊断
}

// Bounds 实现 pkg/span.Interval。
func (s Span) Bounds() (int, int) { return s.Start, s.End }

// Weight 实现 pkg/span.Interval。
func (s Span) Weight() float32 { return s.Score }

func (s Span) Len() int { return s.End - s.Start }

// ValidIn 判断该区间在长度为 textLen 的文本里是否合法。
func (s Span) ValidIn(textLen int) bool {
	return s.Start >= 0 && s.Start < s.End && s.End <= textLen
}

// Text 从原文中切出该区间对应的文本，越界时返回空串。
func (s Span) Text(text string) string {
	if !s.ValidIn(len(text)) {
		return ""
	}
	return text[s.Start:s.End]
}

// NERToken 是 NER 模型输出的单个 token 标注，由 NER 识别器聚合成 Span。
type NERToken struct {
	Type  EntityType
	Tag   BIOTag
	Score float32
	Index int
	Start int // 字节偏移
	End   int // 字节偏移
	Word  string
}
