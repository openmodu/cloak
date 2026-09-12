package confrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileConfig 是 cloak.yaml 的结构，字段名沿用上游 assets/aifw.yaml，
// 便于直接搬运既有配置文件。
type FileConfig struct {
	Port        int              `yaml:"port"`
	APIKeyFile  string           `yaml:"api_key_file"`
	ModelsDir   string           `yaml:"models_dir"`
	HTTPAPIKey  string           `yaml:"http_api_key"`
	LogLevel    string           `yaml:"log_level"`
	Temperature float32          `yaml:"temperature"`
	MaskConfig  map[string]*bool `yaml:"mask_config"`
}

// LoadFile 读取配置文件，文件不存在时返回零值而不是错误——配置文件本就是可选的。
func LoadFile(path string) (FileConfig, error) {
	var cfg FileConfig
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(expandHome(path))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Resolve 按「命令行 > 环境变量 > 配置文件 > 默认值」的顺序取值，
// 与上游 README 里写明的优先级一致。
//
// 环境变量同时认 CLOAK_ 与 AIFW_ 两个前缀，后者是为了让上游用户的既有环境直接可用。
func Resolve(flagVal string, envNames []string, fileVal string, def string) string {
	if flagVal != "" {
		return flagVal
	}
	for _, name := range envNames {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	if fileVal != "" {
		return fileVal
	}
	return def
}

// ResolveInt 是 Resolve 的整数版本，0 视为未设置。
func ResolveInt(flagVal int, envNames []string, fileVal int, def int) int {
	if flagVal != 0 {
		return flagVal
	}
	for _, name := range envNames {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n != 0 {
				return n
			}
		}
	}
	if fileVal != 0 {
		return fileVal
	}
	return def
}

// EnvNames 返回一个配置项在两种前缀下的环境变量名。
func EnvNames(suffix string) []string {
	return []string{"CLOAK_" + suffix, "AIFW_" + suffix}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
