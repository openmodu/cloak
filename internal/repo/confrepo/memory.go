// Package confrepo 提供运行期配置的存取，实现 usecase.ConfigStore。
package confrepo

import (
	"sync/atomic"

	"github.com/openmodu/cloak/internal/types"
)

// Memory 是进程内配置，用原子指针保证热更新与读取无锁并发安全。
type Memory struct {
	mask atomic.Pointer[types.MaskConfig]
}

func New(cfg types.MaskConfig) *Memory {
	m := &Memory{}
	m.SetMask(cfg)
	return m
}

// NewDefault 用默认脱敏开关构造配置。
func NewDefault() *Memory { return New(types.DefaultMaskConfig()) }

func (m *Memory) Mask() types.MaskConfig {
	if c := m.mask.Load(); c != nil {
		return *c
	}
	return types.DefaultMaskConfig()
}

func (m *Memory) SetMask(cfg types.MaskConfig) { m.mask.Store(&cfg) }
