package http

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebAssetsAndAPIAuthorization(t *testing.T) {
	_, h := newTestServer(t, WithAPIKey("test-access"))
	for _, tc := range []struct{ path, contentType, content string }{
		{"/", "text/html", "Share the idea."},
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
