package regexrepo

import "github.com/openmodu/cloak/internal/types"

// PresetSpecsFor 返回某个实体类型的内置规则。
//
// 分数是这套识别的核心调参位：它决定区间能否过阈值（0.5），以及多条规则命中
// 同一段文字时谁胜出。改动前先跑一遍金样本回归。
//
// 全部表达式都在 RE2 语法内（无反向引用、无 lookaround），标准库 regexp 直接编译，
// 不需要引入任何第三方正则引擎。
func PresetSpecsFor(t types.EntityType) []PatternSpec {
	switch t {
	case types.EntityEmailAddress:
		return emailSpecs
	case types.EntityURLAddress:
		return urlSpecs
	case types.EntityPhoneNumber:
		return phoneSpecs
	case types.EntityBankNumber:
		return bankSpecs
	case types.EntityPrivateKey:
		return privKeySpecs
	case types.EntityVerificationCode:
		return vcodeSpecs
	case types.EntityPassword:
		return passwordSpecs
	case types.EntityRandomSeed:
		return seedSpecs
	default:
		return nil
	}
}

var (
	emailSpecs = []PatternSpec{
		{Name: "EMAIL", Pattern: `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`, Score: 0.90},
	}

	urlSpecs = []PatternSpec{
		{Name: "URL", Pattern: `https?://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`, Score: 0.80},
	}

	phoneSpecs = []PatternSpec{
		{Name: "PHONE", Pattern: `\+?\d[\d -]{7,}\d`, Score: 0.70},
	}

	bankSpecs = []PatternSpec{
		// 12-19 位连续数字，覆盖常见卡号与账号长度
		{Name: "BANK", Pattern: `\b\d{12,19}\b`, Score: 0.60},
	}

	privKeySpecs = []PatternSpec{
		{Name: "PEM_PRIVKEY", Pattern: `-----BEGIN (?:OPENSSH|RSA|EC|DSA) PRIVATE KEY-----[\s\S]*?-----END (?:OPENSSH|RSA|EC|DSA) PRIVATE KEY-----`, Score: 0.95},
		// 64 位十六进制，常见的裸私钥长度
		{Name: "HEX_PRIVKEY", Pattern: `\b[0-9a-fA-F]{64}\b`, Score: 0.75},
	}

	vcodeSpecs = []PatternSpec{
		{Name: "VCODE", Pattern: `\b\d{4,8}\b`, Score: 0.50},
		{Name: "VCODE_LABELED_ALNUM", Pattern: `(?i)\b(?:verification\s*code|verify\s*code|otp|2fa\s*code|auth(?:entication)?\s*code)\s*[:=\-]?\s*([A-Za-z0-9]{4,12})`, Score: 0.80, GroupIndex: 1},
	}

	passwordSpecs = []PatternSpec{
		{Name: "PASSWORD_LITERAL", Pattern: `(?i)\bpassword\s*[:=]\s*(\S+)`, Score: 0.40, GroupIndex: 1},
		{Name: "PWD_LITERAL", Pattern: `(?i)\b(?:pwd|pass|passwd|passcode)\s*[:=]\s*(\S+)`, Score: 0.60, GroupIndex: 1},
	}

	seedSpecs = []PatternSpec{
		// seed/mnemonic 后跟 12-24 个小写单词
		{Name: "SEED_PHRASE", Pattern: `(?i)(seed|mnemonic)\s*[:=]?\s*([a-z]+\s+){11,23}[a-z]+`, Score: 0.70},
	}
)
