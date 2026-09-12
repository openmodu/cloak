// Package llmrepo 包装下游大模型调用，实现 usecase.LLMClient。
//
// 只讲 OpenAI 兼容的 /chat/completions 协议：换厂商靠改 base URL，不在代码里分支。
package llmrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openmodu/cloak/internal/types"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 120 * time.Second
)

type Client struct {
	cfg    APIKeyFile
	http   *http.Client
	apiURL string
}

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

func New(cfg APIKeyFile, opts ...Option) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	c := &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: defaultTimeout},
		apiURL: strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewFromFile 从 API key 文件构造客户端。
func NewFromFile(path string, opts ...Option) (*Client, error) {
	cfg, err := LoadAPIKeyFile(path)
	if err != nil {
		return nil, err
	}
	return New(cfg, opts...), nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *Client) Chat(ctx context.Context, req types.ChatRequest) (types.ChatReply, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}
	if model == "" {
		return types.ChatReply{}, fmt.Errorf("llmrepo: 未指定模型，且 api key 文件里没有 openai-model")
	}

	body, err := json.Marshal(chatRequest{
		Model:       model,
		Messages:    []chatMessage{{Role: "user", Content: req.Prompt}},
		Temperature: req.Temperature,
	})
	if err != nil {
		return types.ChatReply{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return types.ChatReply{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return types.ChatReply{}, fmt.Errorf("llmrepo: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return types.ChatReply{}, err
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return types.ChatReply{}, fmt.Errorf("llmrepo: 响应不是合法 JSON (http %d): %s", resp.StatusCode, truncate(string(raw), 200))
	}
	if parsed.Error != nil {
		return types.ChatReply{}, fmt.Errorf("llmrepo: 上游返回错误: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return types.ChatReply{}, fmt.Errorf("llmrepo: http %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	if len(parsed.Choices) == 0 {
		return types.ChatReply{}, fmt.Errorf("llmrepo: 响应里没有 choices")
	}
	return types.ChatReply{Model: parsed.Model, Text: parsed.Choices[0].Message.Content}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
