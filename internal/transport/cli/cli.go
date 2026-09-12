// Package cli 是命令行适配层：解析参数、调用用例、输出结果。
// 这一层不含业务判断，只做协议翻译。
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/openmodu/cloak/internal/repo/llmrepo"
	"github.com/openmodu/cloak/internal/types"
	"github.com/openmodu/cloak/internal/usecase/proxy"
	"io"
	"os"

	"github.com/openmodu/cloak/internal/bootstrap"
	"github.com/openmodu/cloak/internal/repo/confrepo"
)

const usage = `cloak — 发给大模型之前脱敏，拿回结果之后还原

用法:
  cloak mask      [-f 文件]         输出脱敏后的文本
  cloak spans     [-f 文件]         以 JSON 输出识别到的敏感区间
  cloak roundtrip [-f 文件]         脱敏后立即还原，校验与原文一致
  cloak mask --json [-f 文件]       输出 text 与 maskMeta，供以后还原
  cloak restore [-f JSON文件]       从 {text, maskMeta} 还原
  cloak mask-batch [-f JSON文件]    输入 [{text, language}]，输出凭据数组
  cloak restore-batch [-f JSON文件] 输入 [{text, maskMeta}]，输出原文数组
  cloak call --api-key-file 文件    脱敏 → LLM → 还原

可选参数：--config YAML文件、--models-dir 目录、--language 语言。

不带 -f 时从标准输入读取。
`

// Run 执行一次命令，返回进程退出码。
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	cmd := args[0]
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch cmd {
	case "mask", "spans", "roundtrip", "restore", "mask-batch", "restore-batch", "call":
	default:
		fmt.Fprintf(stderr, "未知命令: %s\n%s", cmd, usage)
		return 2
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "", "输入文件，缺省从标准输入读取")
	configPath := fs.String("config", "", "YAML 配置")
	modelsDir := fs.String("models-dir", "", "NER 模型目录")
	language := fs.String("language", "", "语言，空或 auto 自动检测")
	jsonOutput := fs.Bool("json", false, "输出可还原 JSON")
	keyFile := fs.String("api-key-file", "", "LLM 配置")
	model := fs.String("model", "", "模型")
	temperature := fs.String("temperature", "", "温度")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	fileCfg, err := confrepo.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	resolvedTemperature, err := confrepo.ResolveTemperature(*temperature, fileCfg.Temperature)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	text, err := readInput(*file, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "读取输入失败:", err)
		return 1
	}

	app, err := bootstrap.InitApp(bootstrap.Config{
		ModelsDir:  confrepo.ResolvePath(*modelsDir, confrepo.EnvNames("MODELS_DIR"), fileCfg.ModelsDir, ""),
		ENModelID:  confrepo.Resolve("", confrepo.EnvNames("EN_MODEL_ID"), fileCfg.ENModelID, ""),
		ZHModelID:  confrepo.Resolve("", confrepo.EnvNames("ZH_MODEL_ID"), fileCfg.ZHModelID, ""),
		MaskConfig: fileCfg.MaskConfig,
	})
	if err != nil {
		fmt.Fprintln(stderr, "初始化失败:", err)
		return 1
	}

	ctx := context.Background()
	if cmd == "restore" || cmd == "mask-batch" || cmd == "restore-batch" {
		if err := runJSON(ctx, app, cmd, text, *language, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if cmd == "call" {
		client, err := llmrepo.NewFromFile(confrepo.Resolve(*keyFile, confrepo.EnvNames("API_KEY_FILE"), fileCfg.APIKeyFile, ""))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		res, err := proxy.New(app.Masker, app.Restorer, client).Call(ctx, proxy.Request{Text: text, Model: *model, Temperature: resolvedTemperature, Language: types.Language(*language)})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprint(stdout, res.Text)
		return 0
	}
	switch cmd {
	case "mask":
		masked, meta, err := app.Masker.MaskWithLanguage(ctx, text, types.Language(*language))
		if err != nil {
			fmt.Fprintln(stderr, "脱敏失败:", err)
			return 1
		}
		if *jsonOutput {
			encoded, err := meta.EncodeAIFW()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if err := json.NewEncoder(stdout).Encode(jsonText{Text: masked, MaskMeta: encoded}); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		} else {
			fmt.Fprint(stdout, masked)
		}
	case "spans":
		_, meta, err := app.Masker.MaskWithLanguage(ctx, text, types.Language(*language))
		if err != nil {
			fmt.Fprintln(stderr, "识别失败:", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(spansView(text, meta.Items)); err != nil {
			fmt.Fprintln(stderr, "序列化失败:", err)
			return 1
		}
	case "roundtrip":
		masked, meta, err := app.Masker.MaskWithLanguage(ctx, text, types.Language(*language))
		if err != nil {
			fmt.Fprintln(stderr, "脱敏失败:", err)
			return 1
		}
		restored, err := app.Restorer.Restore(ctx, masked, meta)
		if err != nil {
			fmt.Fprintln(stderr, "还原失败:", err)
			return 1
		}
		fmt.Fprintf(stdout, "--- masked ---\n%s\n--- restored ---\n%s\n", masked, restored)
		if restored != text {
			fmt.Fprintln(stderr, "还原结果与原文不一致")
			return 1
		}
		fmt.Fprintf(stderr, "往返一致，共脱敏 %d 处\n", len(meta.Items))
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "未知命令: %s\n\n%s", cmd, usage)
		return 2
	}
	return 0
}

func readInput(file string, stdin io.Reader) (string, error) {
	if file == "" {
		b, err := io.ReadAll(stdin)
		return string(b), err
	}
	b, err := os.ReadFile(file)
	return string(b), err
}
