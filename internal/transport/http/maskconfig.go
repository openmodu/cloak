package http

import "github.com/openmodu/cloak/internal/types"

// maskConfigKeys 是 /api/config 请求里的开关名到实体类型的映射。
// 这些名字是对外接口的一部分，改动会破坏调用方。
var maskConfigKeys = map[string]types.EntityType{
	"maskAddress":          types.EntityPhysicalAddress,
	"maskEmail":            types.EntityEmailAddress,
	"maskOrganization":     types.EntityOrganization,
	"maskUserName":         types.EntityUserName,
	"maskPhoneNumber":      types.EntityPhoneNumber,
	"maskBankNumber":       types.EntityBankNumber,
	"maskPayment":          types.EntityPayment,
	"maskVerificationCode": types.EntityVerificationCode,
	"maskPassword":         types.EntityPassword,
	"maskRandomSeed":       types.EntityRandomSeed,
	"maskPrivateKey":       types.EntityPrivateKey,
	"maskUrl":              types.EntityURLAddress,
}

// applyMaskConfig 把请求里的开关合并进现有配置。
// maskAll 先整体置位，再让单项开关覆盖它，于是「全关但留下邮箱」可以一次请求表达完。
func applyMaskConfig(cur types.MaskConfig, req map[string]*bool) types.MaskConfig {
	if all, ok := req["maskAll"]; ok && all != nil {
		if *all {
			cur = types.EnableAllMaskConfig()
		} else {
			cur = types.DisableAllMaskConfig()
		}
	}
	for key, typ := range maskConfigKeys {
		if v, ok := req[key]; ok && v != nil {
			cur = cur.Set(typ, *v)
		}
	}
	return cur
}
