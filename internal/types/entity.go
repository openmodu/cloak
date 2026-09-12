package types

// EntityType 是受保护的敏感实体类型。取值与上游 aifw core 的 EntityType 枚举
// 逐一对应，String() 的返回值直接用作占位符里的 TAG，因此不可随意改名。
type EntityType uint8

const (
	EntityNone EntityType = iota
	EntityPhysicalAddress
	EntityEmailAddress
	EntityOrganization
	EntityUserName
	EntityPhoneNumber
	EntityBankNumber
	EntityPayment
	EntityVerificationCode
	EntityPassword
	EntityRandomSeed
	EntityPrivateKey
	EntityURLAddress

	entityMax
)

var entityNames = [entityMax]string{
	EntityNone:             "None",
	EntityPhysicalAddress:  "PHYSICAL_ADDRESS",
	EntityEmailAddress:     "EMAIL_ADDRESS",
	EntityOrganization:     "ORGANIZATION",
	EntityUserName:         "USER_NAME",
	EntityPhoneNumber:      "PHONE_NUMBER",
	EntityBankNumber:       "BANK_NUMBER",
	EntityPayment:          "PAYMENT",
	EntityVerificationCode: "VERIFICATION_CODE",
	EntityPassword:         "PASSWORD",
	EntityRandomSeed:       "RANDOM_SEED",
	EntityPrivateKey:       "PRIVATE_KEY",
	EntityURLAddress:       "URL_ADDRESS",
}

func (t EntityType) String() string {
	if t >= entityMax {
		return "UNKNOWN"
	}
	return entityNames[t]
}

func (t EntityType) Valid() bool { return t > EntityNone && t < entityMax }

// ParseEntityType 从 String() 的输出反解出实体类型。
func ParseEntityType(s string) (EntityType, bool) {
	for i, name := range entityNames {
		if name == s {
			return EntityType(i), true
		}
	}
	return EntityNone, false
}

// AllEntityTypes 返回全部可脱敏的实体类型。
func AllEntityTypes() []EntityType {
	out := make([]EntityType, 0, entityMax-1)
	for t := EntityPhysicalAddress; t < entityMax; t++ {
		out = append(out, t)
	}
	return out
}

// BIOTag 是 NER 模型输出的 BIO 标注。
type BIOTag uint8

const (
	TagNone BIOTag = iota
	TagBegin
	TagInside
)
