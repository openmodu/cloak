package llmrepo

import (
	"encoding/json"
	"fmt"
	"os"
)

// APIKeyFile 是 LLM 配置文件的格式：
//
//	{
//	  "openai-api-key": "xxxx",
//	  "openai-base-url": "https://api.openai.com/v1",
//	  "openai-model": "gpt-4o-mini"
//	}
//
// 连字符与下划线两种写法都接受。
type APIKeyFile struct {
	APIKey  string
	BaseURL string
	Model   string
}

// LoadAPIKeyFile 读取并解析配置文件。
func LoadAPIKeyFile(path string) (APIKeyFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return APIKeyFile{}, fmt.Errorf("read api key file: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return APIKeyFile{}, fmt.Errorf("parse api key file %s: %w", path, err)
	}
	cfg := APIKeyFile{
		APIKey:  pick(raw, "openai-api-key", "openai_api_key"),
		BaseURL: pick(raw, "openai-base-url", "openai_base_url"),
		Model:   pick(raw, "openai-model", "openai_model"),
	}
	if cfg.APIKey == "" {
		return cfg, fmt.Errorf("api key file %s: 缺少 openai-api-key", path)
	}
	return cfg, nil
}

func pick(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
