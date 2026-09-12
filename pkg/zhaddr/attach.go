package zhaddr

// canRightAttach 判断候选层级能否接在当前地址链的右边（更细的方向），
// 返回接上之后的新位图，不能接则返回 0。
//
// 基本规则是层级严格相邻（cand 比当前最细层恰好细一级）；其余都是白名单跳级，
// 每条白名单都对应一类真实地址写法。
func canRightAttach(currentBits uint32, cand AddrLevel, text string, curStart, curEnd, candStart, candEnd int) uint32 {
	low := lowestRankInBits(currentBits)
	candR := levelRank(cand)

	// 链上第一个 token：任何层级都接受
	if low == 0 {
		return currentBits | bitFromLevel(cand)
	}
	// 严格相邻
	if candR+1 == low {
		return currentBits | bitFromLevel(cand)
	}

	// 跳级白名单：L11 -> L7，例如「香港 + 铜锣湾」
	if low == levelRank(L11CountryRegion) && candR == levelRank(L7Township) {
		if onlyLightBetween(text, curEnd, candStart, 4) &&
			endsWithAny(text, curStart, curEnd, []string{"香港"}) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L7 -> L3，例如「科兴科学园 + C座」
	if low == levelRank(L7Township) && candR == levelRank(L3Building) {
		if onlyLightBetween(text, curEnd, candStart, 4) &&
			endsWithAny(text, curStart, curEnd, parkSuffixes) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L5 -> L7，例如「8-2号 + 科兴科学园」
	if low == levelRank(L5HouseNo) && candR == levelRank(L7Township) {
		if endsWithAny(text, candStart, candEnd, parkSuffixes) {
			if (candStart <= curEnd && candEnd > curEnd) || nearCharsBetween(text, curEnd, candStart, 4) {
				return currentBits | bitFromLevel(cand)
			}
		}
	}
	// 跳级白名单：L6 -> L4，道路后直接跟 POI
	if low == levelRank(L6Road) && candR == levelRank(L4POI) {
		if onlyLightBetween(text, curEnd, candStart, 4) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L5 -> L2，门牌号后直接跟楼层
	if low == levelRank(L5HouseNo) && candR == levelRank(L2Floor) {
		if nearCharsBetween(text, curEnd, candStart, 4) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L4 -> L6，允许重叠吞并。
	// 针对「沙田银城 + 街 = 沙田银城街」这类：候选道路只多出一个道路后缀。
	if low == levelRank(L4POI) && candR == levelRank(L6Road) {
		if curEnd+roadSuffixAt(text, curEnd) == candEnd {
			// 清掉 L4 位，否则后面的 L5 接不上（L5 不能接在 L4 之后）
			return (currentBits | bitFromLevel(cand)) &^ bitL4
		}
	}
	// 跳级白名单：L4 -> L2，POI 后直接跟楼层
	if low == levelRank(L4POI) && candR == levelRank(L2Floor) {
		if nearCharsBetween(text, curEnd, candStart, 4) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L4 -> L1，POI 后直接跟房间
	if low == levelRank(L4POI) && candR == levelRank(L1UnitRoom) {
		if nearCharsBetween(text, curEnd, candStart, 5) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L3 -> L1，楼栋后直接跟房间
	if low == levelRank(L3Building) && candR == levelRank(L1UnitRoom) {
		if nearCharsBetween(text, curEnd, candStart, 6) {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L8 -> L6，区县后直接跟道路，允许重叠吞并
	if low == levelRank(L8District) && candR == levelRank(L6Road) {
		if candStart <= curEnd && candEnd > curEnd {
			return currentBits | bitFromLevel(cand)
		}
	}
	// 跳级白名单：L9 -> L6，城市后直接跟道路，允许重叠吞并
	if low == levelRank(L9City) && candR == levelRank(L6Road) {
		if candStart <= curEnd && candEnd > curEnd {
			return currentBits | bitFromLevel(cand)
		}
	}
	return 0
}

// parkSuffixes 是园区类后缀，L7 相关的两条跳级白名单共用。
var parkSuffixes = []string{
	"科技园", "科学园", "工业园", "工业区", "产业园",
	"科技園", "科學園", "工業園", "工業區", "產業園",
}

// canLeftAttach 判断候选层级能否接在当前地址链的左边（更粗的方向）。
func canLeftAttach(currentBits uint32, cand AddrLevel, text string, curStart, candStart, candEnd int) uint32 {
	high := highestRankInBits(currentBits)
	candL := levelRank(cand)
	if high == 0 {
		return currentBits // 链上还没有 token
	}
	if candL == high+1 {
		return currentBits | bitFromLevel(cand)
	}

	// 跳级白名单：L6 -> L8，针对「新界」「九龙」这类没有区县后缀的地名
	if high == levelRank(L6Road) && candL == levelRank(L8District) {
		if endsWithAny(text, candStart, candEnd, districtNames) &&
			onlyLightBetween(text, candEnd, curStart, 4) {
			return currentBits | bitFromLevel(cand)
		}
	}
	return 0
}
