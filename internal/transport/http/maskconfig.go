package http

import "github.com/openmodu/cloak/internal/types"

func applyMaskConfig(cur types.MaskConfig, req map[string]*bool) types.MaskConfig {
	return types.ApplyMaskConfig(cur, req)
}
