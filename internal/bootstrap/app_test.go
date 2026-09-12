package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openmodu/cloak/internal/types"
)

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 金样本直接取自上游 aifw 仓库的 tests/*.anonymized.expected.txt。
// 这两份样本里的人名、公司名、中文地址都没有被脱敏，说明产出时 NER 未参与，
// 正好是纯正则路径的预期输出——因此可以用来逐字节验证 Go 版与上游的一致性。
func TestMaskMatchesUpstreamGolden(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		items int
	}{
		{"en_pii.txt", "en_pii.masked.golden.txt", 11},
		{"zh_pii.txt", "zh_pii.masked.golden.txt", 7},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			app, err := InitApp(Config{})
			if err != nil {
				t.Fatal(err)
			}
			got, meta, err := app.Masker.Mask(context.Background(), read(t, c.in))
			if err != nil {
				t.Fatal(err)
			}
			if want := read(t, c.want); got != want {
				t.Fatalf("脱敏结果与上游金样本不一致\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
			if len(meta.Items) != c.items {
				t.Fatalf("want %d masked items, got %d", c.items, len(meta.Items))
			}
		})
	}
}

func TestRoundTripRestoresOriginal(t *testing.T) {
	app, err := InitApp(Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, name := range []string{"en_pii.txt", "zh_pii.txt"} {
		t.Run(name, func(t *testing.T) {
			in := read(t, name)
			masked, meta, err := app.Masker.Mask(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := app.Restorer.Restore(ctx, masked, meta)
			if err != nil {
				t.Fatal(err)
			}
			if restored != in {
				t.Fatal("还原结果与原文不一致")
			}
		})
	}
}

// 关掉开关时文本应当原样通过，用来确认配置真的接到了用例上。
func TestConfigSwitchesAreWired(t *testing.T) {
	app, err := InitApp(Config{})
	if err != nil {
		t.Fatal(err)
	}
	in := read(t, "en_pii.txt")

	app.Conf.SetMask(app.Conf.Mask().Disable(types.EntityEmailAddress))
	masked, _, err := app.Masker.Mask(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(masked, "test.user+alias@example.com") {
		t.Fatal("关闭邮箱脱敏后原始邮箱应当保留")
	}
}
