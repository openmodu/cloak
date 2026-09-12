package regexrepo

import (
	"fmt"
	"strings"
)

// Rust's default character classes are Unicode; Go's Perl classes are ASCII.
const unicodeSpace = `\x{0009}-\x{000D}\x{0020}\x{0085}\x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

func unicodeClasses(expr string) (string, error) {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(expr); i++ {
		if !inClass && strings.HasPrefix(expr[i:], `[\s\S]`) {
			b.WriteString(`(?s:.)`)
			i += 5
			continue
		}
		c := expr[i]
		if c == '\\' && i+1 < len(expr) {
			i++
			escaped := expr[i]
			value, negate := "", false
			switch escaped {
			case 'd', 'D':
				value = `\p{Nd}`
				negate = escaped == 'D'
			case 's', 'S':
				value = unicodeSpace
				negate = escaped == 'S'
			case 'w', 'W':
				value = wordClass
				negate = escaped == 'W'
			}
			if value == "" {
				b.WriteByte(c)
				b.WriteByte(escaped)
				continue
			}
			if inClass {
				if negate {
					return "", fmt.Errorf("negated Unicode class inside character class is unsupported: %s", expr)
				}
				b.WriteString(value)
			} else {
				b.WriteByte('[')
				if negate {
					b.WriteByte('^')
				}
				b.WriteString(value)
				b.WriteByte(']')
			}
			continue
		}
		if c == '[' {
			inClass = true
		}
		if c == ']' {
			inClass = false
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}
