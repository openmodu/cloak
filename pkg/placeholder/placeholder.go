// Package placeholder 负责脱敏占位符的生成。
//
// 格式是 __PII_<TAG>_<ID>__，例如 __PII_EMAIL_ADDRESS_1__。
// TAG 只是一个不透明字符串，本包不关心它代表哪种实体。
//
// 占位符只被生成、从不被解析：还原时由凭据重新生成同样的文本再去回复里查找。
// 这样即使大模型在回复里重排、复制或丢弃了占位符，还原也不会错位。
package placeholder

import (
	"strconv"
	"strings"
)

const (
	prefix = "__PII_"
	suffix = "__"
)

// Render 生成占位符文本。
func Render(tag string, id uint32) string {
	var b strings.Builder
	b.Grow(len(prefix) + len(tag) + 12 + len(suffix))
	b.WriteString(prefix)
	b.WriteString(tag)
	b.WriteByte('_')
	b.WriteString(strconv.FormatUint(uint64(id), 10))
	b.WriteString(suffix)
	return b.String()
}
