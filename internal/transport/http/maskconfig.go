package http

import "github.com/openmodu/cloak/internal/types"

// maskConfigKeys 是 /api/config 请求里的开关名到实体类型的映射，
// 名字与上游 docs/oneaifw_services_api.md 完全一致。
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
// maskAll 先整体置位，再让单项开关覆盖它——与上游的处理顺序一致。
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
