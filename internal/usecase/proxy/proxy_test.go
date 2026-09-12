package proxy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/internal/usecase/masker"
	"github.com/openmodu/cloak/internal/usecase/restorer"
)

type stubRecognizer struct{ spans []types.Span }

func (stubRecognizer) Name() string { return "stub" }

func (s stubRecognizer) Recognize(context.Context, string, types.Language) ([]types.Span, error) {
	return s.spans, nil
}

// captureLLM 记下真正送给模型的文本，用来确认原文没有泄漏出去。
type captureLLM struct {
	got   string
	reply string
	err   error
}

func (c *captureLLM) Chat(_ context.Context, req types.ChatRequest) (types.ChatReply, error) {
	c.got = req.Prompt
	if c.err != nil {
		return types.ChatReply{}, c.err
	}
	return types.ChatReply{Text: c.reply}, nil
}

func newProxy(llm usecase.LLMClient) *Proxy {
	m := masker.New(masker.WithRecognizers(stubRecognizer{spans: []types.Span{
		{Type: types.EntityEmailAddress, Start: 11, End: 17, Score: 0.9},
	}}))
	return New(m, restorer.New(), llm)
}

func TestCallSendsMaskedTextAndRestoresReply(t *testing.T) {
	llm := &captureLLM{reply: "回信到 __PII_EMAIL_ADDRESS_1__ 即可"}
	res, err := newProxy(llm).Call(context.Background(), Request{Text: "my mail is a@b.io"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(llm.got, "a@b.io") {
		t.Fatalf("原文泄漏给了模型: %q", llm.got)
	}
	if !strings.Contains(llm.got, "__PII_EMAIL_ADDRESS_1__") {
		t.Fatalf("送出的应当是脱敏文本: %q", llm.got)
	}
	if res.Text != "回信到 a@b.io 即可" {
		t.Fatalf("got %q", res.Text)
	}
}

// 模型调用失败时不能吞掉错误，更不能把原文当结果返回。
func TestCallPropagatesLLMError(t *testing.T) {
	wantErr := errors.New("上游 429")
	_, err := newProxy(&captureLLM{err: wantErr}).Call(context.Background(), Request{Text: "my mail is a@b.io"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestCallWithoutLLM(t *testing.T) {
	if _, err := newProxy(nil).Call(context.Background(), Request{Text: "x"}); err == nil {
		t.Fatal("没有配置 LLM 时应当报错")
	}
}
