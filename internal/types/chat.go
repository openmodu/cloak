package types

// ChatRequest / ChatReply 是与下游大模型交互的最小载体，
// 刻意不绑定任何厂商的字段，协议细节由 repo 层的实现负责翻译。
type ChatRequest struct {
	Model       string
	Prompt      string
	Temperature float32
}

type ChatReply struct {
	Model string
	Text  string
}
