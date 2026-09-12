package zhaddr

import "math/bits"

// AddrLevel 是地址的层级，数字越小越细。
type AddrLevel uint8

const (
	levelNone AddrLevel = iota
	L1UnitRoom
	L2Floor
	L3Building
	L4POI
	L5HouseNo
	L6Road
	L7Township
	L8District
	L9City
	L10Province
	L11CountryRegion
)

var levelNames = map[AddrLevel]string{
	L1UnitRoom:       "L1_unit_room",
	L2Floor:          "L2_floor",
	L3Building:       "L3_building",
	L4POI:            "L4_poi",
	L5HouseNo:        "L5_house_no",
	L6Road:           "L6_road",
	L7Township:       "L7_township",
	L8District:       "L8_district",
	L9City:           "L9_city",
	L10Province:      "L10_province",
	L11CountryRegion: "L11_country_region",
}

func (l AddrLevel) String() string {
	if s, ok := levelNames[l]; ok {
		return s
	}
	return "unknown"
}

// 各层级在位图里占一位，从第 8 位起排。低 8 位留给将来可能加的其它标志。
const levelBitOffset = 8

const levelBitsLen = int(L11CountryRegion) - int(L1UnitRoom) + 1

const levelBitsMask uint32 = ((1 << levelBitsLen) - 1) << levelBitOffset

const (
	bitL1  uint32 = 1 << (levelBitOffset + iota) // 房间/单元
	bitL2                                        // 楼层
	bitL3                                        // 楼栋
	bitL4                                        // POI
	bitL5                                        // 门牌号
	bitL6                                        // 道路
	bitL7                                        // 乡镇/街道
	bitL8                                        // 区县
	bitL9                                        // 城市
	bitL10                                       // 省
	bitL11                                       // 国家/地区
)

func levelRank(l AddrLevel) int { return int(l) }

func bitFromLevel(l AddrLevel) uint32 {
	return 1 << (uint32(l) + levelBitOffset - 1)
}

// highestRankInBits 返回位图里最粗的层级，没有则返回 0。
func highestRankInBits(b uint32) int {
	checked := b & levelBitsMask
	if checked == 0 {
		return 0
	}
	return bits.Len32(checked) - 1 - (levelBitOffset - 1)
}

// lowestRankInBits 返回位图里最细的层级，没有则返回 0。
func lowestRankInBits(b uint32) int {
	checked := b & levelBitsMask
	if checked == 0 {
		return 0
	}
	return bits.TrailingZeros32(checked) - (levelBitOffset - 1)
}
