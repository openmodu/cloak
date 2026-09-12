package types

var MaskConfigKeys = map[string]EntityType{
	"maskAddress": EntityPhysicalAddress, "maskEmail": EntityEmailAddress,
	"maskOrganization": EntityOrganization, "maskUserName": EntityUserName,
	"maskPhoneNumber": EntityPhoneNumber, "maskBankNumber": EntityBankNumber,
	"maskPayment": EntityPayment, "maskVerificationCode": EntityVerificationCode,
	"maskPassword": EntityPassword, "maskRandomSeed": EntityRandomSeed,
	"maskPrivateKey": EntityPrivateKey, "maskUrl": EntityURLAddress,
}

// ApplyMaskConfig applies the global switch first, then individual overrides.
func ApplyMaskConfig(cur MaskConfig, patch map[string]*bool) MaskConfig {
	if all := patch["maskAll"]; all != nil {
		if *all {
			cur = EnableAllMaskConfig()
		} else {
			cur = DisableAllMaskConfig()
		}
	}
	for key, typ := range MaskConfigKeys {
		if v := patch[key]; v != nil {
			cur = cur.Set(typ, *v)
		}
	}
	return cur
}
