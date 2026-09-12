package http

import (
	"context"
	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase/proxy"
	"testing"
)

type captureLLM struct{ req types.ChatRequest }

func (c *captureLLM) Chat(_ context.Context, r types.ChatRequest) (types.ChatReply, error) {
	c.req = r
	return types.ChatReply{Text: r.Prompt}, nil
}

func TestCallOverridesAndDefaultTemperature(t *testing.T) {
	s, _ := newTestServer(t, WithTemperature(.7))
	client := &captureLLM{}
	chosen := ""
	s.proxyFactory = func(path string) (*proxy.Proxy, error) {
		chosen = path
		return proxy.New(s.masker, s.restorer, client), nil
	}
	for _, tc := range []struct {
		body string
		want float32
	}{
		{`{"text":"a@b.io","apiKeyFile":"chosen.json"}`, .7},
		{`{"text":"a@b.io","apiKeyFile":"chosen.json","temperature":0}`, 0},
	} {
		code, body := do(t, s.Handler(), "POST", "/api/call", tc.body, nil)
		if code != 200 || chosen != "chosen.json" || client.req.Temperature != tc.want {
			t.Fatalf("%d %v req=%+v", code, body, client.req)
		}
		if client.req.Prompt == "a@b.io" {
			t.Fatal("sent unmasked prompt")
		}
	}
}

func TestErrorObjectAndCaseInsensitiveBearer(t *testing.T) {
	_, h := newTestServer(t, WithAPIKey("secret"))
	code, body := do(t, h, "POST", "/api/mask_text", `{"text":"hello"}`, nil)
	if code != 401 {
		t.Fatal(code)
	}
	e, ok := body["error"].(map[string]any)
	if !ok || e["message"] == nil || e["code"] != nil {
		t.Fatalf("%v", body)
	}
	code, _ = do(t, h, "POST", "/api/mask_text", `{"text":"hello"}`, map[string]string{"Authorization": "bEaReR secret"})
	if code != 200 {
		t.Fatal(code)
	}
}
