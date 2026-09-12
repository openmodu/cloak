package pathsafe

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T) (base string, outside string) {
	t.Helper()
	root := t.TempDir()
	base = filepath.Join(root, "keys")
	outside = filepath.Join(root, "secret")
	for _, d := range []string{base, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "ok.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "stolen.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	return base, outside
}

func TestWithinAcceptsFilesInsideBase(t *testing.T) {
	base, _ := setup(t)
	got, err := Within(base, "ok.json")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "ok.json" {
		t.Fatalf("got %q", got)
	}
	// 目标不存在也应当通过——存不存在由调用方去报，不是越界
	if _, err := Within(base, "sub/missing.json"); err != nil {
		t.Fatalf("不存在的路径不该被判越界: %v", err)
	}
}

func TestWithinRejectsEscapes(t *testing.T) {
	base, _ := setup(t)
	for _, rel := range []string{
		"../secret/stolen.json",
		"..",
		"../../etc/passwd",
		"sub/../../secret/stolen.json",
		"/etc/passwd",
		"/etc/shadow",
		"c:windows",
		"",
	} {
		if _, err := Within(base, rel); err == nil {
			t.Fatalf("%q 应当被拒绝", rel)
		}
	}
}

// base 里放一个指向外部的符号链接，不解析就会被绕过。
func TestWithinRejectsSymlinkEscape(t *testing.T) {
	base, outside := setup(t)
	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("无法创建符号链接:", err)
	}
	if _, err := Within(base, "link/stolen.json"); !errors.Is(err, ErrOutsideBase) {
		t.Fatalf("符号链接逃逸应当被拒绝，得到 %v", err)
	}
}

func TestWithinRejectsUnusableBase(t *testing.T) {
	if _, err := Within(filepath.Join(t.TempDir(), "nope"), "x.json"); err == nil {
		t.Fatal("基准目录不存在时应当报错")
	}
}
