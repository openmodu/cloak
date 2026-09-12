package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openmodu/cloak/internal/repo/confrepo"
	"github.com/openmodu/cloak/internal/repo/regexrepo"
	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/internal/usecase/masker"
	"github.com/openmodu/cloak/internal/usecase/proxy"
	"github.com/openmodu/cloak/internal/usecase/restorer"
	"github.com/openmodu/cloak/pkg/pathsafe"
)

func newTestServer(t *testing.T, opts ...Option) (*Server, http.Handler) {
	t.Helper()
	set, err := regexrepo.NewDefaultSet()
	if err != nil {
		t.Fatal(err)
	}
	rs := make([]usecase.Recognizer, 0, len(set))
	for _, r := range set {
		rs = append(rs, r)
	}
	conf := confrepo.NewDefault()
	m := masker.New(masker.WithRecognizers(rs...), masker.WithConfigStore(conf))
	s := New(m, restorer.New(), conf, opts...)
	return s, s.Handler()
}

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var parsed map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("响应不是合法 JSON: %s", rec.Body.String())
		}
	}
	return rec.Code, parsed
}

func TestHealth(t *testing.T) {
	_, h := newTestServer(t)
	code, body := do(t, h, "GET", "/api/health", "", nil)
	if code != 200 || body["status"] != "ok" {
		t.Fatalf("got %d %v", code, body)
	}
}

// mask 产出的 maskMeta 原样回传就能还原，服务端不留存任何东西。
func TestMaskRestoreRoundTrip(t *testing.T) {
	_, h := newTestServer(t)
	const text = "My email is test@example.com and my phone is 18744325579."

	code, body := do(t, h, "POST", "/api/mask_text",
		`{"text":`+jsonString(text)+`,"language":"en"}`, nil)
	if code != 200 {
		t.Fatalf("mask 失败: %d %v", code, body)
	}
	out := body["output"].(map[string]any)
	masked := out["text"].(string)
	meta := out["maskMeta"].(string)

	if strings.Contains(masked, "test@example.com") {
		t.Fatalf("邮箱没有被脱敏: %s", masked)
	}
	if !strings.Contains(masked, "__PII_EMAIL_ADDRESS_1__") {
		t.Fatalf("占位符格式不对: %s", masked)
	}

	code, body = do(t, h, "POST", "/api/restore_text",
		`{"text":`+jsonString(masked)+`,"maskMeta":`+jsonString(meta)+`}`, nil)
	if code != 200 {
		t.Fatalf("restore 失败: %d %v", code, body)
	}
	if got := body["output"].(map[string]any)["text"].(string); got != text {
		t.Fatalf("还原结果不一致:\n got %q\nwant %q", got, text)
	}
}

func TestMaskRestoreBatch(t *testing.T) {
	_, h := newTestServer(t)
	code, body := do(t, h, "POST", "/api/mask_text_batch",
		`[{"text":"a@b.io"},{"text":"pwd: hunter2"}]`, nil)
	if code != 200 {
		t.Fatalf("got %d %v", code, body)
	}
	outs := body["output"].([]any)
	if len(outs) != 2 {
		t.Fatalf("want 2 outputs, got %d", len(outs))
	}

	var items []string
	for _, o := range outs {
		m := o.(map[string]any)
		items = append(items, `{"text":`+jsonString(m["text"].(string))+`,"maskMeta":`+jsonString(m["maskMeta"].(string))+`}`)
	}
	code, body = do(t, h, "POST", "/api/restore_text_batch", "["+strings.Join(items, ",")+"]", nil)
	if code != 200 {
		t.Fatalf("got %d %v", code, body)
	}
	restored := body["output"].([]any)
	if restored[0].(map[string]any)["text"] != "a@b.io" ||
		restored[1].(map[string]any)["text"] != "pwd: hunter2" {
		t.Fatalf("批量还原结果不对: %v", restored)
	}
}

// /api/config 免重启改开关，改完立刻生效。
func TestConfigTakesEffectImmediately(t *testing.T) {
	_, h := newTestServer(t)
	code, _ := do(t, h, "POST", "/api/config", `{"maskConfig":{"maskEmail":false}}`, nil)
	if code != 200 {
		t.Fatalf("config 失败: %d", code)
	}
	_, body := do(t, h, "POST", "/api/mask_text", `{"text":"a@b.io"}`, nil)
	if got := body["output"].(map[string]any)["text"].(string); got != "a@b.io" {
		t.Fatalf("关掉邮箱脱敏后应当原样返回，实际 %q", got)
	}
}

func TestConfigMaskAllThenOverride(t *testing.T) {
	_, h := newTestServer(t)
	// maskAll 先整体置位，单项开关再覆盖它
	do(t, h, "POST", "/api/config", `{"maskConfig":{"maskAll":false,"maskEmail":true}}`, nil)
	_, body := do(t, h, "POST", "/api/mask_text", `{"text":"a@b.io tel 18744325579"}`, nil)
	got := body["output"].(map[string]any)["text"].(string)
	if !strings.Contains(got, "__PII_EMAIL_ADDRESS_") {
		t.Fatalf("邮箱应当被脱敏: %q", got)
	}
	if strings.Contains(got, "__PII_PHONE_NUMBER_") {
		t.Fatalf("电话应当保持关闭: %q", got)
	}
}

func TestConfigRequiresMaskConfig(t *testing.T) {
	_, h := newTestServer(t)
	if code, _ := do(t, h, "POST", "/api/config", `{}`, nil); code != 400 {
		t.Fatalf("want 400, got %d", code)
	}
}

// 没接 LLM 时 /api/call 要明确回 503，而不是假装成功。
func TestCallWithoutLLM(t *testing.T) {
	_, h := newTestServer(t)
	code, body := do(t, h, "POST", "/api/call", `{"text":"hi"}`, nil)
	if code != 503 {
		t.Fatalf("want 503, got %d %v", code, body)
	}
	if body["error"] == nil {
		t.Fatal("应当带上错误说明")
	}
}

type stubLLM struct{ reply string }

func (s stubLLM) Chat(context.Context, types.ChatRequest) (types.ChatReply, error) {
	return types.ChatReply{Text: s.reply}, nil
}

// 完整链路：脱敏后的文本进模型，模型回的占位符再被还原回去。
func TestCallRestoresLLMReply(t *testing.T) {
	set, _ := regexrepo.NewDefaultSet()
	rs := make([]usecase.Recognizer, 0, len(set))
	for _, r := range set {
		rs = append(rs, r)
	}
	conf := confrepo.NewDefault()
	m := masker.New(masker.WithRecognizers(rs...), masker.WithConfigStore(conf))
	r := restorer.New()
	// 模型原样把占位符还回来
	p := proxy.New(m, r, stubLLM{reply: "联系方式是 __PII_EMAIL_ADDRESS_1__"})

	s := New(m, r, conf, WithProxy(p))
	code, body := do(t, s.Handler(), "POST", "/api/call", `{"text":"my mail is a@b.io"}`, nil)
	if code != 200 {
		t.Fatalf("got %d %v", code, body)
	}
	if got := body["output"].(map[string]any)["text"].(string); got != "联系方式是 a@b.io" {
		t.Fatalf("got %q", got)
	}
}

func TestAuthRejectsWrongKey(t *testing.T) {
	_, h := newTestServer(t, WithAPIKey("secret"))

	if code, _ := do(t, h, "POST", "/api/mask_text", `{"text":"x"}`, nil); code != 401 {
		t.Fatalf("无口令应当 401，实际 %d", code)
	}
	if code, _ := do(t, h, "POST", "/api/mask_text", `{"text":"x"}`,
		map[string]string{"Authorization": "wrong"}); code != 401 {
		t.Fatalf("错口令应当 401，实际 %d", code)
	}
	// 裸 key 与 Bearer 两种写法都接受
	if code, _ := do(t, h, "POST", "/api/mask_text", `{"text":"x"}`,
		map[string]string{"Authorization": "secret"}); code != 200 {
		t.Fatalf("裸 key 应当通过，实际 %d", code)
	}
	if code, _ := do(t, h, "POST", "/api/mask_text", `{"text":"x"}`,
		map[string]string{"Authorization": "Bearer secret"}); code != 200 {
		t.Fatalf("Bearer 写法应当通过，实际 %d", code)
	}
	// 健康检查不需要口令
	if code, _ := do(t, h, "GET", "/api/health", "", nil); code != 200 {
		t.Fatalf("健康检查应当免校验，实际 %d", code)
	}
}

func TestBadRequestBody(t *testing.T) {
	_, h := newTestServer(t)
	if code, _ := do(t, h, "POST", "/api/mask_text", `{not json`, nil); code != 400 {
		t.Fatalf("want 400, got %d", code)
	}
}

// 非法 maskMeta 要报错，不能静默返回原文假装成功。
func TestRestoreWithBadMeta(t *testing.T) {
	_, h := newTestServer(t)
	if code, _ := do(t, h, "POST", "/api/restore_text",
		`{"text":"x","maskMeta":"!!!not-base64!!!"}`, nil); code != 400 {
		t.Fatalf("want 400, got %d", code)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// 请求级 apiKeyFile 默认关闭：不配置目录时，任何指定路径的请求都要被拒。
// 否则等于把「读服务端任意本地文件」的能力交给了调用方。
func TestRequestAPIKeyFileDisabledByDefault(t *testing.T) {
	_, h := newTestServer(t)
	code, body := do(t, h, "POST", "/api/call",
		`{"text":"hi","apiKeyFile":"/etc/passwd"}`, nil)
	if code != 400 {
		t.Fatalf("want 400, got %d %v", code, body)
	}
}

// 启用之后也只接受目录内的相对路径，且错误信息不能透露文件系统细节。
func TestRequestAPIKeyFileIsSandboxed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.json"), []byte(
		`{"openai-api-key":"k","openai-base-url":"http://127.0.0.1:1/v1","openai-model":"m"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotPath string
	_, h := newTestServer(t, WithProxyFactory(func(p string) (*proxy.Proxy, error) {
		resolved, err := pathsafe.Within(dir, p)
		if err != nil {
			return nil, err
		}
		gotPath = resolved
		return nil, errors.New("stop here: 本测试只验证路径解析")
	}))

	for _, tc := range []struct {
		name string
		path string
	}{
		{"绝对路径", "/etc/passwd"},
		{"父目录逃逸", "../../etc/passwd"},
		{"目录外", "../other.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := do(t, h, "POST", "/api/call",
				`{"text":"hi","apiKeyFile":`+jsonString(tc.path)+`}`, nil)
			if code != 400 {
				t.Fatalf("want 400, got %d", code)
			}
			if msg, _ := body["error"].(map[string]any)["message"].(string); strings.Contains(msg, "/etc") ||
				strings.Contains(msg, dir) {
				t.Fatalf("错误信息泄漏了路径: %q", msg)
			}
		})
	}

	// 目录内的相对路径应当被解析到该目录下
	do(t, h, "POST", "/api/call", `{"text":"hi","apiKeyFile":"good.json"}`, nil)
	if gotPath != filepath.Join(mustEval(t, dir), "good.json") {
		t.Fatalf("解析结果不对: %q", gotPath)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return real
}
