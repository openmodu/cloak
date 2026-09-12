package textutil

import "testing"

func TestDetectScript(t *testing.T) {
	cases := []struct {
		name string
		text string
		want Script
	}{
		{"纯英文", "Hello world, this is a test.", ScriptLatin},
		{"简体中文", "我的家庭住址是北京市朝阳区", ScriptHans},
		{"繁体中文", "我的家庭住址是臺北市信義區這裡", ScriptHant},
		{"中英混排按中文处理", "我的 email 是 test@example.com", ScriptHans},
		{"英文里夹一两个汉字仍按英文", "The character 中 appears once in this long English sentence about nothing", ScriptLatin},
		{"空文本", "", ScriptUnknown},
		{"纯数字", "12345", ScriptUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectScript(c.text); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
