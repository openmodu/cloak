package confrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileConfig 是 cloak.yaml 的结构。
type FileConfig struct {
	Port        int              `yaml:"port"`
	APIKeyFile  string           `yaml:"api_key_file"`
	ModelsDir   string           `yaml:"models_dir"`
	HTTPAPIKey  string           `yaml:"http_api_key"`
	LogLevel    string           `yaml:"log_level"`
	Temperature float32          `yaml:"temperature"`
	MaskConfig  map[string]*bool `yaml:"mask_config"`
}

// LoadFile 在未指定路径时使用默认配置；显式路径不存在时报错，避免误用默认脱敏开关。
func LoadFile(path string) (FileConfig, error) {
	var cfg FileConfig
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(expandHome(path))
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Resolve 按「命令行 > 环境变量 > 配置文件 > 默认值」的顺序取值。
// 越靠近调用现场的来源优先级越高，便于临时覆盖而不改配置文件。
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

// EnvNames 返回一个配置项对应的环境变量名。
func EnvNames(suffix string) []string {
	return []string{"CLOAK_" + suffix}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
