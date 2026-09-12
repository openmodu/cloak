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
		// 日文大量使用汉字，只数汉字会把它判成中文，进而套上中文地址规则、
		// 选错模型、还做一次没有意义的简繁转换。假名是排他性信号，必须先看。
		{"日文", "こんにちは、田中さんは東京に住んでいます", ScriptJapanese},
		{"日文片假名", "コンピュータの設定", ScriptJapanese},
		{"日文夹汉字地名", "東京都渋谷区に住んでいます", ScriptJapanese},
		{"韩文", "안녕하세요 서울에 살고 있습니다", ScriptKorean},
		{"韩文夹汉字", "韓國 서울시", ScriptKorean},
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

// 简繁判定：命中繁体专用字即判繁体，否则判简体。
func TestDetectHansHant(t *testing.T) {
	cases := map[string]Script{
		"我住在北京市朝阳区建国路": ScriptHans,
		"我住在臺北市信義區":    ScriptHant,
		"個人資訊與電腦軟體":    ScriptHant,
		"广州市天河区体育东路":   ScriptHans,
		// 通篇没有简繁差异字，两者本来也无区别，判简体
		"我在上海": ScriptHans,
	}
	for text, want := range cases {
		if got := DetectScript(text); got != want {
			t.Fatalf("%q → %v, want %v", text, got, want)
		}
	}
}
