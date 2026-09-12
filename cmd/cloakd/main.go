// Command cloakd 是 OneAIFW Go 版的 HTTP 服务入口。
// 接口与上游 docs/oneaifw_services_api.md 对齐。
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
		configPath = flag.String("config", "", "配置文件路径（YAML），可选")
		port       = flag.Int("port", 0, "监听端口，默认 8844")
		host       = flag.String("host", "", "监听地址，默认 127.0.0.1")
		apiKeyFile = flag.String("api-key-file", "", "LLM API key 文件路径；不给则不接入大模型")
		modelsDir  = flag.String("models-dir", "", "NER 模型根目录；不给则只用正则识别")
		httpAPIKey = flag.String("http-api-key", "", "本服务的访问口令；不给则不校验 Authorization")
	)
	flag.Parse()

	fileCfg, err := confrepo.LoadFile(*configPath)
	if err != nil {
		return err
	}

	// 取值优先级：命令行 > 环境变量 > 配置文件 > 默认值
	addr := fmt.Sprintf("%s:%d",
		confrepo.Resolve(*host, confrepo.EnvNames("HOST"), "", "127.0.0.1"),
		confrepo.ResolveInt(*port, confrepo.EnvNames("PORT"), fileCfg.Port, defaultPort),
	)
	cfg := bootstrap.Config{
		ModelsDir:  confrepo.Resolve(*modelsDir, confrepo.EnvNames("MODELS_DIR"), fileCfg.ModelsDir, ""),
		APIKeyFile: confrepo.Resolve(*apiKeyFile, confrepo.EnvNames("API_KEY_FILE"), fileCfg.APIKeyFile, ""),
		HTTPAPIKey: confrepo.Resolve(*httpAPIKey, confrepo.EnvNames("HTTP_API_KEY"), fileCfg.HTTPAPIKey, ""),
	}

	srv, err := bootstrap.InitServer(cfg)
	if err != nil {
		return err
	}
	if cfg.ModelsDir == "" {
		slog.Warn("未配置 NER 模型目录，只用正则识别；人名、机构名、中文地址不会被脱敏")
	}
	if cfg.APIKeyFile == "" {
		slog.Warn("未配置 LLM API key 文件，/api/call 不可用；脱敏与还原接口照常工作")
	}
	if cfg.HTTPAPIKey == "" {
		slog.Warn("未设置访问口令，任何人都能调用本服务；对外暴露时请设置 --http-api-key")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return srv.ListenAndServe(ctx, addr)
}
