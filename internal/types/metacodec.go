package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Encode 把还原凭据序列化成一个不透明字符串，供跨请求、跨进程传递。
//
// 上游 core 是把内部结构体按二进制布局序列化后交给绑定层再 base64；这里换成
// base64(JSON)，语义一致而且跨语言可读。对调用方来说它同样是不透明的：
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
