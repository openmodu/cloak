package nerrepo

import (
	"strings"

	"github.com/openmodu/cloak/internal/types"
)

// splitLabel 把 "B-PER" 这样的标签拆成核心类别与 BIO 标记。
// 兼容 BIOES 标注：S-（单字实体）视同 B-，E-（实体结尾）视同 I-。
func splitLabel(label string) (core string, tag types.BIOTag) {
	s := strings.TrimSpace(label)
	switch {
	case strings.HasPrefix(s, "B-"), strings.HasPrefix(s, "S-"):
		return s[2:], types.TagBegin
	case strings.HasPrefix(s, "I-"), strings.HasPrefix(s, "E-"):
		return s[2:], types.TagInside
	case s != "":
		return s, types.TagNone
	}
	return "MISC", types.TagNone
}

// labelToEntityType 把模型的标签类别映射成受保护的实体类型。
// 认不出来的一律当作非敏感——宁可漏标，也不要把普通词当成实体打码。
func labelToEntityType(core string) types.EntityType {
	switch strings.ToUpper(core) {
	case "PER", "PERSON":
		return types.EntityUserName
	case "ORG":
		return types.EntityOrganization
	case "LOC", "GPE", "FAC", "ADDRESS":
		return types.EntityPhysicalAddress
	default:
		return types.EntityNone
	}
}
