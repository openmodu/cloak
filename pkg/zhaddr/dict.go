package zhaddr

// 词典与匹配函数逐条对应上游 core/merge_zh_addr.zig 里的同名常量与函数。
// 顺序有意义：较长的后缀必须排在较短的前面，否则会被短的先吃掉。

var countryRegionNames = []string{
	"中国", "中華人民共和國", "中华人民共和国", "中國大陸", "中国大陆",
	"臺灣", "台湾", "香港", "澳門", "澳门",
	"英國", "英国", "美國", "美国", "日本",
}

func matchCountryRegionAt(text string, pos int) int {
	for _, name := range countryRegionNames {
		if matchToken(text, pos, name) {
			return len(name)
		}
	}
	return 0
}

var provinceSuffixes = []string{"省", "自治区", "自治州", "盟", "地区", "特别行政区"}

func provinceSuffixAt(text string, pos int) int {
	for _, s := range provinceSuffixes {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

// citySuffixAt 对「市」特殊处理，避免把「城市」这种普通名词当成行政后缀，
// 例如「新城市廣場」里的那个「市」不应被识别为市级后缀。
func citySuffixAt(text string, pos int) int {
	if matchToken(text, pos, "市") {
		prev := utf8PrevCpStart(text, pos)
		if prev < len(text) && matchToken(text, prev, "城市") {
			return 0
		}
		return len("市")
	}
	return 0
}

var districtSuffixes = []string{"区", "區", "县", "縣", "旗"}

func districtSuffixAt(text string, pos int) int {
	for _, s := range districtSuffixes {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

var districtNames = []string{"新界", "九龙", "九龍"}

func matchDistrictNameAt(text string, pos int) int {
	for _, name := range districtNames {
		if matchToken(text, pos, name) {
			return len(name)
		}
	}
	return 0
}

var townshipSuffixes = []string{
	"街道", "镇", "鎮", "乡", "鄉",
	"开发区", "经济技术开发区", "科技园", "科学园", "工业园",
	"工业区", "产业园", "科技園", "科學園", "工業園",
	"工業區", "產業園",
}

func townshipSuffixAt(text string, pos int) int {
	for _, s := range townshipSuffixes {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

// 港澳与内地的少量片区名，非穷举。
var townshipNames = []string{
	"铜锣湾", "銅鑼灣", "北角", "荃湾", "荃灣", "将军澳", "將軍澳", "青衣", "上环", "上環",
}

func matchTownshipNameAt(text string, pos int) int {
	for _, name := range townshipNames {
		if matchToken(text, pos, name) {
			return len(name)
		}
	}
	return 0
}

var poiSuffixes = []string{
	"广场", "中心", "花园", "花苑", "苑", "城", "天地", "大厦", "大楼", "港", "塔", "廊", "坊", "里", "府",
	"购物公园", "购物艺术馆", "廣場", "花園", "大廈", "大樓", "購物公園", "購物藝術館",
}

// endsWithPoiSuffix 判断 [start,end) 是否以 POI 后缀结尾。
//
// 「城」要特殊处理：若其后紧跟「区/區/县/縣/市」（可跨轻分隔符），
// 说明整段其实是行政区划（如「上城区」），不该当成以「城」结尾的 POI。
func endsWithPoiSuffix(text string, start, end int) bool {
	if end <= start {
		return false
	}
	for _, suffix := range poiSuffixes {
		if end < start+len(suffix) || !matchToken(text, end-len(suffix), suffix) {
			continue
		}
		if suffix == "城" {
			p := end
			for p < len(text) && isASCIILight(text[p]) {
				p++
			}
			if p < len(text) &&
				(matchToken(text, p, "区") || matchToken(text, p, "區") ||
					matchToken(text, p, "县") || matchToken(text, p, "縣") ||
					matchToken(text, p, "市")) {
				return false
			}
		}
		return true
	}
	return false
}

var roadSuffixes = []string{
	// 长后缀在前
	"大道", "大街", "环路", "环线", "道中", "道东", "道西", "道南", "道北",
	"路", "街", "巷", "弄", "里", "道", "胡同", "段", "環路",
	"環線",
}

func roadSuffixAt(text string, pos int) int {
	for _, s := range roadSuffixes {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

// findRoadSuffixInsideEnd 在 [start,end) 内找第一个道路后缀，返回其结束位置，没有则 0。
func findRoadSuffixInsideEnd(text string, start, end int) int {
	for p := start; p < end; p++ {
		if l := roadSuffixAt(text, p); l > 0 {
			return p + l
		}
	}
	return 0
}

// buildingUnitAt 刻意不收「楼/樓」，否则「18楼」会被当成楼栋；
// 「号楼/號樓」算楼栋，单独的「楼/樓」留给 floorUnitAt。
var buildingUnits = []string{"号楼", "号館", "號樓", "館", "栋", "棟", "幢", "座"}

func buildingUnitAt(text string, pos int) int {
	for _, s := range buildingUnits {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

var floorUnits = []string{"层", "層", "樓", "楼"}

func floorUnitAt(text string, pos int) int {
	for _, s := range floorUnits {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}

var roomUnits = []string{"单元", "室", "房"}

func roomUnitAt(text string, pos int) int {
	for _, s := range roomUnits {
		if matchToken(text, pos, s) {
			return len(s)
		}
	}
	return 0
}
