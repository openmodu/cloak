package scripts

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestDownloadRecovery(t *testing.T) {
	for _, tool := range []string{"bash", "curl", "flock", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(err)
		}
	}
	var broken atomic.Bool
	broken.Store(true)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if broken.Load() {
			w.Header().Set("Content-Length", "100")
		}
		_, _ = w.Write([]byte("complete model"))
	}))
	defer server.Close()
	helper, err := filepath.Abs("download.sh")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "model.onnx")
	fetch := func() error {
		cmd := exec.Command("bash", "-c", `source "$1"; fetch "$2" "$3"`, "download-test", helper, server.URL, dest)
		cmd.Env = append(os.Environ(), "CLOAK_DOWNLOAD_RETRIES=0")
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		return err
	}
	if err := fetch(); err == nil {
		t.Fatal("truncated download succeeded")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("partial file published")
	}
	broken.Store(false)
	if err := fetch(); err != nil {
		t.Fatal(err)
	}
	n := requests.Load()
	if err := fetch(); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != n {
		t.Fatal("valid download was not reused")
	}
	if err := os.WriteFile(dest, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := fetch(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest)
	if err != nil || string(b) != "complete model" {
		t.Fatalf("not repaired: %q %v", b, err)
	}
}
