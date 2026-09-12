// Command cloakd 是 cloak 的 HTTP 服务入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/openmodu/cloak/internal/bootstrap"
	"github.com/openmodu/cloak/internal/repo/confrepo"
)

const defaultPort = 8844

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cloakd:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath   = flag.String("config", "", "配置文件路径（YAML），可选")
		port         = flag.Int("port", 0, "监听端口，默认 8844")
		host         = flag.String("host", "", "监听地址，默认 127.0.0.1")
		apiKeyFile   = flag.String("api-key-file", "", "LLM API key 文件路径；不给则不接入大模型")
		modelsDir    = flag.String("models-dir", "", "NER 模型根目录；不给则只用正则识别")
		enModelID    = flag.String("en-model", "", "英文 NER 模型在模型根目录下的相对路径")
		zhModelID    = flag.String("zh-model", "", "中文 NER 模型在模型根目录下的相对路径")
		httpAPIKey   = flag.String("http-api-key", "", "本服务的访问口令；不给则不校验 Authorization")
		apiKeyDir    = flag.String("api-key-dir", "", "允许请求用 apiKeyFile 指定的目录；不给则该功能关闭")
		temperature  = flag.String("temperature", "", "默认 LLM 温度")
		addrFallback = flag.Bool("address-fallback", true,
			"中文地址融合失败时保留 NER 的原始地址区间；关掉会让这类地址整条不脱敏")
		logLevel = flag.String("log-level", "", "日志级别")
	)
	flag.Parse()

	fileCfg, err := confrepo.LoadFile(*configPath)
	if err != nil {
		return err
	}
	resolvedTemperature, err := confrepo.ResolveTemperature(*temperature, fileCfg.Temperature)
	if err != nil {
		return err
	}

	// 取值优先级：命令行 > 环境变量 > 配置文件 > 默认值
	addr := fmt.Sprintf("%s:%d",
		confrepo.Resolve(*host, confrepo.EnvNames("HOST"), "", "127.0.0.1"),
		confrepo.ResolveInt(*port, confrepo.EnvNames("PORT"), fileCfg.Port, defaultPort),
	)
	cfg := bootstrap.Config{
		ModelsDir:       confrepo.ResolvePath(*modelsDir, confrepo.EnvNames("MODELS_DIR"), fileCfg.ModelsDir, ""),
		ENModelID:       confrepo.Resolve(*enModelID, confrepo.EnvNames("EN_MODEL_ID"), fileCfg.ENModelID, ""),
		ZHModelID:       confrepo.Resolve(*zhModelID, confrepo.EnvNames("ZH_MODEL_ID"), fileCfg.ZHModelID, ""),
		APIKeyFile:      confrepo.ResolvePath(*apiKeyFile, confrepo.EnvNames("API_KEY_FILE"), fileCfg.APIKeyFile, ""),
		HTTPAPIKey:      confrepo.Resolve(*httpAPIKey, confrepo.EnvNames("HTTP_API_KEY"), fileCfg.HTTPAPIKey, ""),
		APIKeyDir:       confrepo.ResolvePath(*apiKeyDir, confrepo.EnvNames("API_KEY_DIR"), fileCfg.APIKeyDir, ""),
		MaskConfig:      fileCfg.MaskConfig,
		AddressFallback: addrFallbackFromFlags(addrFallback, fileCfg.AddressFallback),
		Temperature:     resolvedTemperature,
	}
	level := new(slog.Level)
	if value := confrepo.Resolve(*logLevel, confrepo.EnvNames("LOG_LEVEL"), fileCfg.LogLevel, "INFO"); value != "" {
		if err := level.UnmarshalText([]byte(value)); err != nil {
			return fmt.Errorf("log_level: %w", err)
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	srv, err := bootstrap.InitServer(cfg)
	if err != nil {
		return err
	}
	if cfg.ModelsDir == "" {
		slog.Warn("未配置 NER 模型目录，只用正则识别；人名、机构名、中文地址不会被脱敏")
	}
	if cfg.APIKeyFile == "" {
		slog.Warn("未配置默认 LLM API key 文件，/api/call 需要在请求中提供 apiKeyFile")
	}
	if cfg.HTTPAPIKey == "" {
		slog.Warn("未设置访问口令，任何人都能调用本服务；对外暴露时请设置 --http-api-key")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return srv.ListenAndServe(ctx, addr)
}

// addrFallbackFromFlags 只在命令行显式出现 -address-fallback 时才让它生效，
// 否则把决定权交给环境变量与配置文件。flag 包没法直接区分「用了默认值」和
// 「显式传了同样的值」，只能扫一遍已设置的标志。
func addrFallbackFromFlags(flagVal *bool, fileVal *bool) *bool {
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "address-fallback" {
			explicit = true
		}
	})
	var fromFlag *bool
	if explicit {
		fromFlag = flagVal
	}
	v := confrepo.ResolveBool(fromFlag, confrepo.EnvNames("ADDRESS_FALLBACK"), fileVal, true)
	return &v
}
