package types

import "testing"

func TestDefaultMaskConfigMatchesUpstream(t *testing.T) {
	c := DefaultMaskConfig()
	if c.Enabled(EntityPhysicalAddress) {
		t.Fatal("物理地址默认不脱敏")
	}
	for _, typ := range AllEntityTypes() {
		if typ == EntityPhysicalAddress {
			continue
		}
		if !c.Enabled(typ) {
			t.Fatalf("%s 默认应当脱敏", typ)
		}
	}
	// 位序与上游 aifw 的 ENABLE_MASK_*_BIT 对齐：地址是第 0 位，邮箱第 1 位
	if bitOf(EntityPhysicalAddress) != 1<<0 || bitOf(EntityEmailAddress) != 1<<1 {
		t.Fatal("位序与上游不一致")
	}
}

func TestEnableDisableIsValueSemantics(t *testing.T) {
	base := DisableAllMaskConfig()
	on := base.Enable(EntityEmailAddress)
	if base.Enabled(EntityEmailAddress) {
		t.Fatal("原配置不应被就地修改")
	}
	if !on.Enabled(EntityEmailAddress) {
		t.Fatal("副本应当已开启")
	}
	if on.Disable(EntityEmailAddress).Enabled(EntityEmailAddress) {
		t.Fatal("关闭失败")
	}
}

func TestEntityTypeNameRoundTrip(t *testing.T) {
	for _, typ := range AllEntityTypes() {
		got, ok := ParseEntityType(typ.String())
		if !ok || got != typ {
			t.Fatalf("%s 反解失败", typ)
		}
	}
}
