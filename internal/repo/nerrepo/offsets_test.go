package nerrepo

import "testing"

func sliceAll(text string, offsets [][2]int) []string {
	out := make([]string, len(offsets))
	for i, o := range offsets {
		out[i] = text[o[0]:o[1]]
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

// 偏移不取自分词器，而是拿 token 串回原文按游标查找。
func TestComputeOffsetsBasic(t *testing.T) {
	text := "Acme Corporation is here"
	tokens := []string{"acme", "corp", "oration", "is"}
	eq(t, sliceAll(text, computeOffsetsFromTokens(text, tokens)),
		[]string{"Acme", "Corp", "oration", "is"})
}

// 子词前缀要先剥掉再定位。
func TestComputeOffsetsStripsSubwordPrefix(t *testing.T) {
	text := "Corporation"
	tokens := []string{"corp", "##oration"}
	eq(t, sliceAll(text, computeOffsetsFromTokens(text, tokens)),
		[]string{"Corp", "oration"})
}

// 分词器会去掉重音，原文里还留着，必须靠去重音的退路对上。
func TestComputeOffsetsHandlesAccents(t *testing.T) {
	text := "José lives here"
	tokens := []string{"jose", "lives"}
	eq(t, sliceAll(text, computeOffsetsFromTokens(text, tokens)),
		[]string{"José", "lives"})
}

// 连接符同样会被分词器吃掉。
func TestComputeOffsetsHandlesConnectorPunct(t *testing.T) {
	text := "O'Brien called"
	tokens := []string{"obrien", "called"}
	got := sliceAll(text, computeOffsetsFromTokens(text, tokens))
	if got[0] != "O'Brien" {
		t.Fatalf("got %q", got)
	}
}

// 中文按字定位，偏移必须落在字符边界上。
func TestComputeOffsetsChinese(t *testing.T) {
	text := "北京市朝阳区"
	tokens := []string{"北", "京", "市", "朝", "阳", "区"}
	eq(t, sliceAll(text, computeOffsetsFromTokens(text, tokens)),
		[]string{"北", "京", "市", "朝", "阳", "区"})
}

// 定位不到的 token 退化成零宽区间，不能让偏移乱跑。
func TestComputeOffsetsUnfoundToken(t *testing.T) {
	text := "hello world"
	offsets := computeOffsetsFromTokens(text, []string{"hello", "zzzz", "world"})
	if offsets[1][0] != offsets[1][1] {
		t.Fatalf("找不到的 token 应当是零宽区间: %+v", offsets[1])
	}
	if text[offsets[2][0]:offsets[2][1]] != "world" {
		t.Fatalf("后续 token 定位错了: %+v", offsets[2])
	}
}

// 大小写转换后字节长度会变的字符不能让偏移整体错位。
func TestComputeOffsetsWithLengthChangingCase(t *testing.T) {
	text := "K and john" // U+212A KELVIN SIGN，小写后是单字节 k
	offsets := computeOffsetsFromTokens(text, []string{"john"})
	if got := text[offsets[0][0]:offsets[0][1]]; got != "john" {
		t.Fatalf("got %q", got)
	}
}
