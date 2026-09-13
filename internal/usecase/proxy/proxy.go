// Package proxy 实现完整链路用例：脱敏 → 调用大模型 → 还原。
package proxy

import (
	"context"
	"fmt"
	"time"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/internal/usecase/masker"
	"github.com/openmodu/cloak/internal/usecase/restorer"
)

// Proxy 把三步串起来。它只依赖 LLMClient 接口，不关心下游是哪家模型。
type Proxy struct {
	masker   *masker.Masker
	restorer *restorer.Restorer
	llm      usecase.LLMClient
}

func New(m *masker.Masker, r *restorer.Restorer, llm usecase.LLMClient) *Proxy {
	return &Proxy{masker: m, restorer: r, llm: llm}
}

// Request 是一次完整调用的入参。
type Request struct {
	Language    types.Language
	Text        string
	Model       string
	Temperature float32
}

// Result 除了最终文本，也带回中间产物，方便排查「模型把占位符弄丢了」这类问题。
type Result struct {
	Timings    Timings
	Text       string
	Masked     string
	LLMReply   string
	MaskedMeta *types.MaskMeta
}

// Timings measures server wall time in milliseconds, including upstream I/O.
// Total excludes HTTP request decoding and response encoding/transfer.
type Timings struct {
	MaskMS    float64 `json:"maskMs"`
	LLMMS     float64 `json:"llmMs"`
	RestoreMS float64 `json:"restoreMs"`
	TotalMS   float64 `json:"totalMs"`
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// Call 执行 脱敏 → 调用 → 还原。任一步失败都不会把原文送出去。
func (p *Proxy) Call(ctx context.Context, req Request) (Result, error) {
	started := time.Now()
	var timings Timings
	if p.llm == nil {
		return Result{}, fmt.Errorf("proxy: 未配置 LLM 客户端")
	}
	stage := time.Now()
	masked, meta, err := p.masker.MaskWithLanguage(ctx, req.Text, req.Language)
	timings.MaskMS = milliseconds(time.Since(stage))
	if err != nil {
		return Result{}, fmt.Errorf("proxy: 脱敏失败: %w", err)
	}

	stage = time.Now()
	reply, err := p.llm.Chat(ctx, types.ChatRequest{
		Model:       req.Model,
		Prompt:      masked,
		Temperature: req.Temperature,
	})
	timings.LLMMS = milliseconds(time.Since(stage))
	if err != nil {
		return Result{}, fmt.Errorf("proxy: 调用大模型失败: %w", err)
	}

	stage = time.Now()
	restored, err := p.restorer.Restore(ctx, reply.Text, meta)
	timings.RestoreMS = milliseconds(time.Since(stage))
	if err != nil {
		return Result{}, fmt.Errorf("proxy: 还原失败: %w", err)
	}
	timings.TotalMS = milliseconds(time.Since(started))
	return Result{Text: restored, Masked: masked, LLMReply: reply.Text, MaskedMeta: meta, Timings: timings}, nil
}
