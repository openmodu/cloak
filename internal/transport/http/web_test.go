package http

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestWebAssetsAndAPIAuthorization(t *testing.T) {
	_, h := newTestServer(t, WithAPIKey("test-access"))
	for _, tc := range []struct{ path, contentType, content string }{
		{"/", "text/html", `id="workspace-form"`},
		{"/assets/app.css", "text/css", "@media"},
		{"/assets/app.js", "text/javascript", "/api/mask_text"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), tc.contentType) || !strings.Contains(rec.Body.String(), tc.content) {
				t.Fatalf("unexpected asset response: %d %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatal("missing security headers")
			}
		})
	}
	for _, path := range []string{"/missing", "/assets/missing.js", "/api/missing", "/web/index.html"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 404 {
			t.Fatalf("%s: want 404, got %d", path, rec.Code)
		}
	}
	code, _ := do(t, h, "POST", "/api/mask_text", `{"text":"test@example.com"}`, nil)
	if code != 401 {
		t.Fatalf("UI must not bypass API authorization: %d", code)
	}
	code, _ = do(t, h, "POST", "/api/mask_text", `{"text":"test@example.com"}`, map[string]string{"Authorization": "Bearer test-access"})
	if code != 200 {
		t.Fatalf("authenticated API failed: %d", code)
	}
}

// app.js 靠 getElementById 找元素，HTML 里少一个 id 只会在浏览器里静默报错，
// 服务端测试照样全绿。这里把两边对起来：脚本引用的每个 id 都必须在页面上存在。
func TestWebScriptElementIDsExist(t *testing.T) {
	html, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	script, err := webFiles.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}

	refs := regexp.MustCompile(`el\('([a-zA-Z0-9-]+)'\)`).FindAllSubmatch(script, -1)
	if len(refs) == 0 {
		t.Fatal("没有从 app.js 里解析出任何元素引用，正则可能已失效")
	}

	seen := map[string]bool{}
	for _, m := range refs {
		id := string(m[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		if !strings.Contains(string(html), `id="`+id+`"`) {
			t.Errorf("app.js 引用了 id=%q，但 index.html 里没有", id)
		}
	}
}
