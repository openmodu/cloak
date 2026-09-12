package zhaddr

// token 是窗口内识别出的一个地址成分。
type token struct {
	level AddrLevel
	start int
	end   int
}

// findChunkStart 从 endPos 往左回退至多 maxChars 个字符，遇到分隔符即停，
// 用来圈出一个行政区划或道路名的大致起点。
func findChunkStart(text string, startPos, endPos, maxChars int) int {
	p := endPos
	consumed := 0
	for p > startPos && consumed < maxChars {
		prev := utf8PrevCpStart(text, p)
		if isASCIILight(text[prev]) || heavySepAt(text, prev) > 0 {
			break
		}
		consumed += p - prev
		p = prev
	}
	return p
}

// adjustAdminRoadChunkStart 收紧起点，使其不跨过上一级行政区划或道路后缀。
//
// 例如「江苏省南京市鼓楼区广州路」中：
//   - 在「市」处识别 L9 时，应当从「南」开始，而不是「江」；
//   - 在「区」处识别 L8 时，应当从「鼓」开始；
//   - 在「路」处识别 L6 时，应当从「广」开始。
func adjustAdminRoadChunkStart(text string, level AddrLevel, s0, suffixPos int) int {
	var checkFn func(string, int) int
	switch level {
	case L10Province:
		checkFn = matchCountryRegionAt
	case L9City:
		checkFn = provinceSuffixAt
	case L8District:
		checkFn = citySuffixAt
	case L7Township:
		checkFn = districtSuffixAt
	case L6Road:
		checkFn = townshipSuffixAt
	default:
		return s0
	}

	lastEnd := s0
	for p := s0; p < suffixPos; {
		if adminLen := checkFn(text, p); adminLen > 0 {
			lastEnd = p + adminLen
			p += adminLen
			continue
		}
		step := utf8CpLenAt(text, p)
		if step == 0 {
			break
		}
		p += step
	}
	return lastEnd
}

// tokenizeWindow 扫描 [start,end) 并切出地址成分，返回出现过的层级位图、
// token 列表，以及最后一个 token 的结束位置（没有 token 时原样返回传入的 newEnd）。
//
// 识别顺序即优先级，先粗后细；命中后把游标推到该 token 末尾继续扫。
func tokenizeWindow(text string, start, end, newEnd int) (uint32, []token, int) {
	var bits uint32
	var tokens []token
	if end <= start || end > len(text) {
		return 0, nil, newEnd
	}
	limit := end

	for i := start; i < limit; {
		if isASCIILight(text[i]) {
			i++
			continue
		}

		// L11 国家/地区
		if l := matchCountryRegionAt(text, i); l > 0 {
			e := i + l
			bits |= bitL11
			tokens = append(tokens, token{L11CountryRegion, i, e})
			newEnd, i = e, e
			continue
		}
		// L10 省
		if l := provinceSuffixAt(text, i); l > 0 {
			e := i + l
			s := adjustAdminRoadChunkStart(text, L10Province, findChunkStart(text, start, i, 32), i)
			bits |= bitL10
			tokens = append(tokens, token{L10Province, s, e})
			newEnd, i = e, e
			continue
		}
		// L9 市
		if l := citySuffixAt(text, i); l > 0 {
			e := i + l
			s := adjustAdminRoadChunkStart(text, L9City, findChunkStart(text, start, i, 24), i)
			bits |= bitL9
			tokens = append(tokens, token{L9City, s, e})
			newEnd, i = e, e
			continue
		}
		// L8 区县：先认地名，再认后缀
		if l := matchDistrictNameAt(text, i); l > 0 {
			e := i + l
			bits |= bitL8
			tokens = append(tokens, token{L8District, i, e})
			newEnd, i = e, e
			continue
		}
		if l := districtSuffixAt(text, i); l > 0 {
			e := i + l
			s := adjustAdminRoadChunkStart(text, L8District, findChunkStart(text, start, i, 24), i)
			bits |= bitL8
			tokens = append(tokens, token{L8District, s, e})
			newEnd, i = e, e
			continue
		}
		// L7 乡镇/街道/园区
		if l := matchTownshipNameAt(text, i); l > 0 {
			e := i + l
			bits |= bitL7
			tokens = append(tokens, token{L7Township, i, e})
			newEnd, i = e, e
			continue
		}
		if l := townshipSuffixAt(text, i); l > 0 {
			e := i + l
			s := adjustAdminRoadChunkStart(text, L7Township, findChunkStart(text, start, i, 24), i)
			bits |= bitL7
			tokens = append(tokens, token{L7Township, s, e})
			newEnd, i = e, e
			continue
		}
		// L6 道路
		if l := roadSuffixAt(text, i); l > 0 {
			e := i + l
			s := adjustAdminRoadChunkStart(text, L6Road, findChunkStart(text, start, i, 32), i)
			bits |= bitL6
			tokens = append(tokens, token{L6Road, s, e})
			newEnd, i = e, e
			continue
		}
		// L5 门牌号：「号/號」前面要有数字；「号楼/號樓」归楼栋，不在这里处理
		if (matchToken(text, i, "号") || matchToken(text, i, "號")) &&
			!(matchToken(text, i, "号楼") || matchToken(text, i, "號樓")) {
			p, hasDigit, dstart := i, false, i
			for steps := 0; p > start && steps < 8; steps++ {
				p--
				if isASCIILight(text[p]) {
					continue
				}
				if isDigit(text[p]) {
					hasDigit = true
					dstart = p
					// 数字一直往左吃完，这里不受窗口起点限制
					for dstart > 0 && isDigit(text[dstart-1]) {
						dstart--
					}
					break
				}
			}
			if hasDigit {
				e := i + len("号")
				if matchToken(text, i, "號") {
					e = i + len("號")
				}
				// 吃掉「之3」「-2」这类门牌尾巴
				q := e
				for q < limit && isASCIILight(text[q]) {
					q++
				}
				if q < limit && matchToken(text, q, "之") {
					q += len("之")
					for q < limit && isASCIILight(text[q]) {
						q++
					}
					d0 := q
					for q < limit && isDigit(text[q]) {
						q++
					}
					if q > d0 {
						e = q
					}
				} else if q < limit && text[q] == '-' {
					q++
					for q < limit && isASCIILight(text[q]) {
						q++
					}
					d1 := q
					for q < limit && isDigit(text[q]) {
						q++
					}
					if q > d1 {
						e = q
					}
				}
				bits |= bitL5
				tokens = append(tokens, token{L5HouseNo, dstart, e})
				newEnd, i = e, e
				continue
			}
			// 前面没有数字，继续往下当别的成分试
		}
		// L4 POI：从当前位置往右最多吃 16 个字，遇到 POI 后缀即成
		if poiEnd, found := scanPOI(text, i, limit); found {
			// 若 [i,poiEnd) 内部含有道路后缀，说明这段很可能是「德輔道中恒生大廈」
			// 这种「道路+POI」的连写。此时拆成两个 token，让道路先被识别出来。
			if roadEnd := findRoadSuffixInsideEnd(text, i, poiEnd); roadEnd == 0 {
				bits |= bitL4
				tokens = append(tokens, token{L4POI, i, poiEnd})
			} else {
				bits |= bitL6 | bitL4
				tokens = append(tokens, token{L6Road, i, roadEnd})
				tokens = append(tokens, token{L4POI, roadEnd, poiEnd})
			}
			newEnd, i = poiEnd, poiEnd
			continue
		}
		// L3 楼栋：单位词前面要有数字或字母（如「12栋」「C座」）
		if l := buildingUnitAt(text, i); l > 0 {
			if dstart, ok := scanBackForDigitOrAlpha(text, start, i); ok {
				e := i + l
				bits |= bitL3
				tokens = append(tokens, token{L3Building, dstart, e})
				newEnd, i = e, e
				continue
			}
		}
		// L2 楼层：「18层」，或英文写法「F3」
		if l := floorUnitAt(text, i); l > 0 {
			if dstart, ok := scanBackForDigit(text, start, i); ok {
				e := i + l
				bits |= bitL2
				tokens = append(tokens, token{L2Floor, dstart, e})
				newEnd, i = e, e
				continue
			}
		} else if text[i] == 'F' {
			q := i + 1
			d0 := q
			for q < limit && isDigit(text[q]) {
				q++
			}
			if q > d0 {
				bits |= bitL2
				tokens = append(tokens, token{L2Floor, i, q})
				newEnd, i = q, q
				continue
			}
		}
		// L1 「之3」这类尾巴（如「18楼之3」）
		if matchToken(text, i, "之") {
			q := i + len("之")
			for q < limit && isASCIILight(text[q]) {
				q++
			}
			d0 := q
			for q < limit && isDigit(text[q]) {
				q++
			}
			if q > d0 {
				bits |= bitL1
				tokens = append(tokens, token{L1UnitRoom, i, q})
				newEnd, i = q, q
				continue
			}
		}
		// L1 房间/单元
		if l := roomUnitAt(text, i); l > 0 {
			if dstart, ok := scanBackForDigitOrAlphaStopAtLight(text, start, i); ok {
				e := i + l
				bits |= bitL1
				tokens = append(tokens, token{L1UnitRoom, dstart, e})
				newEnd, i = e, e
				continue
			}
		}
		i++
	}

	tokens, bits = dropTokensFinerThanLast(tokens, bits)
	return bits, tokens, newEnd
}

// scanPOI 从 i 往右找 POI 后缀，最多吃 16 个字符。
func scanPOI(text string, i, limit int) (poiEnd int, found bool) {
	const maxName = 16
	j, consumed := i, 0
	for j < limit && consumed < maxName {
		if heavySepAt(text, j) > 0 {
			break
		}
		b := text[j]
		switch {
		case isDigit(b):
			return i, false
		case isASCIIAlpha(b):
			j++
			consumed++
		case b&0x80 != 0:
			j += utf8CpLenAt(text, j)
			consumed++
		default:
			return i, false
		}
		if j > i && endsWithPoiSuffix(text, i, j) {
			return j, true
		}
	}
	return i, false
}

// scanBackForDigitOrAlpha 往左找数字或字母作为楼栋编号的起点，跳过轻分隔符。
func scanBackForDigitOrAlpha(text string, start, i int) (int, bool) {
	p := i
	for steps := 0; p > start && steps < 8; steps++ {
		p--
		if isASCIILight(text[p]) {
			continue // 楼栋允许跨过空格回看
		}
		if isDigit(text[p]) {
			d := p
			for d > start && (isDigit(text[d-1]) || isASCIIAlpha(text[d-1])) {
				d--
			}
			return d, true
		}
		if isASCIIAlpha(text[p]) {
			d := p
			for d > start && isASCIIAlpha(text[d-1]) {
				d--
			}
			return d, true
		}
	}
	return i, false
}

// scanBackForDigit 往左找楼层数字，遇到轻分隔符即停（楼层必须紧贴数字）。
func scanBackForDigit(text string, start, i int) (int, bool) {
	p := i
	for steps := 0; p > start && steps < 8; steps++ {
		p--
		if isASCIILight(text[p]) {
			break
		}
		if isDigit(text[p]) {
			d := p
			for d > start && isDigit(text[d-1]) {
				d--
			}
			return d, true
		}
	}
	return i, false
}

// scanBackForDigitOrAlphaStopAtLight 往左找房号的数字或字母，遇到轻分隔符即停。
func scanBackForDigitOrAlphaStopAtLight(text string, start, i int) (int, bool) {
	p := i
	for steps := 0; p > start && steps < 8; steps++ {
		p--
		if isASCIILight(text[p]) {
			break
		}
		if isDigit(text[p]) {
			d := p
			for d > start && isDigit(text[d-1]) {
				d--
			}
			return d, true
		}
		if isASCIIAlpha(text[p]) {
			d := p
			for d > start && isASCIIAlpha(text[d-1]) {
				d--
			}
			return d, true
		}
	}
	return i, false
}

// dropTokensFinerThanLast 丢掉排在最后一个 token 之前、但层级比它更细的 token，
// 并清掉对应的位。窗口里出现这种倒挂多半是误切，留着会干扰层级链的判定。
func dropTokensFinerThanLast(tokens []token, bits uint32) ([]token, uint32) {
	if len(tokens) < 2 {
		return tokens, bits
	}
	lastLevel := tokens[len(tokens)-1].level
	for idx := len(tokens) - 2; idx >= 0; idx-- {
		if levelRank(tokens[idx].level) < levelRank(lastLevel) {
			bits &^= bitFromLevel(tokens[idx].level)
			tokens = append(tokens[:idx], tokens[idx+1:]...)
		}
	}
	return tokens, bits
}
