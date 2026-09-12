package confrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 配置文件里写 ~/... 是常态。不展开的话只会得到一句「文件不存在」，
// 让人误以为是模型没装好——这正是它值得一个测试的原因。
func TestResolvePathExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	got := ResolvePath("", nil, "~/.cloak/models", "")
	if want := filepath.Join(home, ".cloak", "models"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := ResolvePath("", nil, "~", ""); got != home {
		t.Fatalf("got %q, want %q", got, home)
	}
}

func TestResolvePathLeavesOtherFormsAlone(t *testing.T) {
	for _, in := range []string{"/abs/path", "relative/path", "", "./x", "~user/x", "a~b"} {
		if got := ResolvePath("", nil, in, ""); got != in {
			t.Fatalf("%q 不该被改写，得到 %q", in, got)
		}
	}
}

// 命令行与环境变量来源同样要展开。
func TestResolvePathExpandsAllSources(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("CLOAK_TEST_PATH", "~/from-env")
	if got := ResolvePath("", []string{"CLOAK_TEST_PATH"}, "", ""); !strings.HasPrefix(got, home) {
		t.Fatalf("环境变量来源没展开: %q", got)
	}
	if got := ResolvePath("~/from-flag", nil, "", ""); !strings.HasPrefix(got, home) {
		t.Fatalf("命令行来源没展开: %q", got)
	}
}
