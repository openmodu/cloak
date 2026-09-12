package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Encode 把还原凭据序列化成一个不透明字符串，供跨请求、跨进程传递。
//
// 编码用 base64(JSON)：跨语言可读，也便于排查问题。对调用方来说它是不透明的，
// 拿到什么就原样传回来。
//
// 注意：凭据里含有原文，必须与脱敏后的文本同等看待，不要写进日志或转给第三方。
func (m *MaskMeta) Encode() (string, error) {
	if m == nil {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("encode mask meta: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeMaskMeta 是 Encode 的逆操作。空串解出空凭据，还原时等价于原样返回。
func DecodeMaskMeta(s string) (*MaskMeta, error) {
	if s == "" {
		return &MaskMeta{}, nil
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode mask meta: 不是合法的 base64: %w", err)
	}
	var m MaskMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode mask meta: %w", err)
	}
	return &m, nil
}
