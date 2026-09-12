package types

// MaskConfig 用位图表示每类实体是否需要脱敏。位序与上游 aifw core 的
// ENABLE_MASK_*_BIT 一致，方便两端互通配置。
type MaskConfig struct {
	Bits uint32 `json:"bits"`
}

func bitOf(t EntityType) uint32 {
	if !t.Valid() {
		return 0
	}
	return 1 << (uint32(t) - 1)
}

// AllMaskBits 是全部实体都脱敏时的位图。
func AllMaskBits() uint32 {
	var bits uint32
	for _, t := range AllEntityTypes() {
		bits |= bitOf(t)
	}
	return bits
}

// DefaultMaskConfig 与上游默认值一致：除物理地址外全部开启。
// 地址默认关闭是因为它极易误伤正常语句，需要使用者显式打开。
func DefaultMaskConfig() MaskConfig {
	return MaskConfig{Bits: AllMaskBits() &^ bitOf(EntityPhysicalAddress)}
}

func EnableAllMaskConfig() MaskConfig  { return MaskConfig{Bits: AllMaskBits()} }
func DisableAllMaskConfig() MaskConfig { return MaskConfig{} }

func (c MaskConfig) Enabled(t EntityType) bool { return c.Bits&bitOf(t) != 0 }

// Enable / Disable 返回修改后的副本，保持值语义，避免共享配置被就地改写。
func (c MaskConfig) Enable(t EntityType) MaskConfig {
	return MaskConfig{Bits: c.Bits | bitOf(t)}
}

func (c MaskConfig) Disable(t EntityType) MaskConfig {
	return MaskConfig{Bits: c.Bits &^ bitOf(t)}
}

func (c MaskConfig) Set(t EntityType, on bool) MaskConfig {
	if on {
		return c.Enable(t)
	}
	return c.Disable(t)
}
