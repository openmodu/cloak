package zhaddr

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "重新生成金样本文件")

const (
	datasetPath = "../../testdata/zh_address_dataset.txt"
	goldenPath  = "../../testdata/zh_address.golden.txt"
)

// dumpDataset 把数据集每一行按「整行作为种子」跑一遍融合，输出固定格式。
// 规则一改，金样本里的分数、偏移或文本就会变，diff 一眼看得出改动影响了哪几条。
func dumpDataset(t *testing.T) string {
	t.Helper()
	f, err := os.Open(datasetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		res := Merge(line, []Seed{{Kind: SeedAddress, Start: 0, End: len(line), Score: 0.9}})
		if len(res) == 0 {
			b.WriteString("none\n")
			continue
		}
		for _, r := range res {
			fmt.Fprintf(&b, "%.4f\t%d\t%d\t%s\n", r.Score, r.Start, r.End, line[r.Start:r.End])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestDatasetMatchesGolden(t *testing.T) {
	got := dumpDataset(t)
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("已重新生成", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("与金样本不一致\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// 抽查几条有代表性的地址，说明规则到底在做什么。
func TestMergeRepresentativeCases(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string // 空串表示不该产出地址
	}{
		{"门牌号即达隐私阈值", "北京市朝阳区建国路 88 号", "北京市朝阳区建国路 88 号"},
		{"只到区县不算敏感", "上海市浦东新区", ""},
		{"只到国家地区不算敏感", "香港特別行政區", ""},
		{"POI 加楼层算敏感", "成都市高新区天府大道100号环球中心5楼", "成都市高新区天府大道100号环球中心5楼"},
		{"繁体同样识别", "香港中環皇后大道中99號中環中心18樓1803室", "香港中環皇后大道中99號中環中心18樓1803室"},
		{"门牌号带之字尾", "新北市板桥区文化路二段182号18楼之3", "新北市板桥区文化路二段182号18楼之3"},
		{"英文楼层写法", "杭州市滨江区江南大道228号滨江大厦F3 305室", "杭州市滨江区江南大道228号滨江大厦F3 305室"},
		// 种子是整行，起点自然从行首算起；这里要验的是**不向右吞并**姓名等后续内容
		{"不吞并地址之后的其它信息", "我住在珠海市香洲区中山路234号，我的名字是张信哲", "我住在珠海市香洲区中山路234号"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := Merge(c.text, []Seed{{Kind: SeedAddress, Start: 0, End: len(c.text), Score: 0.9}})
			if c.want == "" {
				if len(res) != 0 {
					t.Fatalf("不该产出地址，实际: %q", c.text[res[0].Start:res[0].End])
				}
				return
			}
			if len(res) != 1 {
				t.Fatalf("want 1 result, got %d", len(res))
			}
			if got := c.text[res[0].Start:res[0].End]; got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// 分数随最细层级递增，房间号级别应当高于门牌号级别。
func TestScoreIncreasesWithDetail(t *testing.T) {
	coarse := "北京市朝阳区建国路88号"
	fine := "北京市朝阳区建国路88号国贸中心A座1208室"
	rc := Merge(coarse, []Seed{{Kind: SeedAddress, Start: 0, End: len(coarse), Score: 0.9}})
	rf := Merge(fine, []Seed{{Kind: SeedAddress, Start: 0, End: len(fine), Score: 0.9}})
	if len(rc) != 1 || len(rf) != 1 {
		t.Fatalf("got %d / %d", len(rc), len(rf))
	}
	if rf[0].Score <= rc[0].Score {
		t.Fatalf("更细的地址分数应当更高: %v vs %v", rf[0].Score, rc[0].Score)
	}
}

// 相邻的地址碎片先粘起来再融合。
func TestAdjacentSeedsAreJoined(t *testing.T) {
	text := "北京市朝阳区 建国路88号国贸中心A座1208室"
	seeds := []Seed{
		{Kind: SeedAddress, Start: 0, End: len("北京市朝阳区"), Score: 0.9},
		{Kind: SeedAddress, Start: len("北京市朝阳区 "), End: len(text), Score: 0.9},
	}
	res := Merge(text, seeds)
	if len(res) != 1 || res[0].Start != 0 || res[0].End != len(text) {
		t.Fatalf("got %+v", res)
	}
}

// 非地址、非机构的种子一律忽略。
func TestOtherSeedsIgnored(t *testing.T) {
	text := "北京市朝阳区建国路88号"
	if res := Merge(text, []Seed{{Kind: SeedOther, Start: 0, End: len(text), Score: 0.9}}); len(res) != 0 {
		t.Fatalf("got %+v", res)
	}
}

// ContainsPrivateDetail 只看出现过什么成分，不受拼接顺序影响。
func TestContainsPrivateDetail(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"有门牌号", "北京市朝阳区建国路88号", true},
		{"POI 加楼层", "成都市高新区环球中心5楼", true},
		{"只到区县", "上海市浦东新区", false},
		{"只到国家地区", "香港特別行政區", false},
		{"只到道路", "北京市朝阳区建国路", false},
		// 关键一条：园区名排在门牌号之后，拼接时会被判层级倒挂删掉，
		// 但这段文字本身确实含门牌号，必须判为敏感。
		{"门牌号后跟园区名", "深圳市南山区科技南十二路8-2号科兴科学园", true},
		{"空串", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ContainsPrivateDetail(c.text, 0, len(c.text)); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// 越界区间不能 panic。
func TestContainsPrivateDetailBadRange(t *testing.T) {
	text := "北京市朝阳区建国路88号"
	for _, c := range [][2]int{{-1, 5}, {0, len(text) + 10}, {5, 5}, {8, 3}} {
		if ContainsPrivateDetail(text, c[0], c[1]) {
			t.Fatalf("越界区间 %v 应当返回 false", c)
		}
	}
}
