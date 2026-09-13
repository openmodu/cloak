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

const usage = `cloak — Mask sensitive data before sending it to an LLM, then restore the response

Usage:
  cloak mask          [-f FILE]       Output masked text
  cloak spans         [-f FILE]       Output detected sensitive spans as JSON
  cloak roundtrip     [-f FILE]       Mask, restore, and verify a lossless round trip
  cloak mask --json   [-f FILE]       Output text and maskMeta for later restoration
  cloak restore       [-f JSON_FILE]  Restore from {text, maskMeta}
  cloak mask-batch    [-f JSON_FILE]  Mask [{text, language}] into restorable records
  cloak restore-batch [-f JSON_FILE]  Restore [{text, maskMeta}] into original texts
  cloak call --api-key-file FILE      Mask -> LLM -> restore

Options: --config YAML_FILE, --models-dir DIR, --language LANGUAGE.

Reads from standard input when -f is omitted.
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
		fmt.Fprintf(stderr, "Unknown command: %s\n%s", cmd, usage)
		return 2
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "", "Input file (defaults to standard input)")
	configPath := fs.String("config", "", "YAML configuration file")
	modelsDir := fs.String("models-dir", "", "NER model directory")
	language := fs.String("language", "", "Language (empty or auto enables detection)")
	jsonOutput := fs.Bool("json", false, "Output restorable JSON")
	keyFile := fs.String("api-key-file", "", "LLM configuration file")
	model := fs.String("model", "", "LLM model")
	temperature := fs.String("temperature", "", "Sampling temperature")
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
		fmt.Fprintln(stderr, "Failed to read input:", err)
		return 1
	}

	app, err := bootstrap.InitApp(bootstrap.Config{
		ModelsDir:       confrepo.ResolvePath(*modelsDir, confrepo.EnvNames("MODELS_DIR"), fileCfg.ModelsDir, ""),
		ENModelID:       confrepo.Resolve("", confrepo.EnvNames("EN_MODEL_ID"), fileCfg.ENModelID, ""),
		ZHModelID:       confrepo.Resolve("", confrepo.EnvNames("ZH_MODEL_ID"), fileCfg.ZHModelID, ""),
		MaskConfig:      fileCfg.MaskConfig,
		AddressFallback: addressFallback(fileCfg.AddressFallback),
	})
	if err != nil {
		fmt.Fprintln(stderr, "Initialization failed:", err)
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
			fmt.Fprintln(stderr, "Masking failed:", err)
			return 1
		}
		if *jsonOutput {
			encoded, err := meta.EncodeBinary()
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
			fmt.Fprintln(stderr, "Detection failed:", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(spansView(text, meta.Items)); err != nil {
			fmt.Fprintln(stderr, "Serialization failed:", err)
			return 1
		}
	case "roundtrip":
		masked, meta, err := app.Masker.MaskWithLanguage(ctx, text, types.Language(*language))
		if err != nil {
			fmt.Fprintln(stderr, "Masking failed:", err)
			return 1
		}
		restored, err := app.Restorer.Restore(ctx, masked, meta)
		if err != nil {
			fmt.Fprintln(stderr, "Restoration failed:", err)
			return 1
		}
		fmt.Fprintf(stdout, "--- masked ---\n%s\n--- restored ---\n%s\n", masked, restored)
		if restored != text {
			fmt.Fprintln(stderr, "Restored text does not match the original")
			return 1
		}
		fmt.Fprintf(stderr, "Round trip verified: %d spans masked\n", len(meta.Items))
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n\n%s", cmd, usage)
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

// addressFallback 让 CLI 也能用环境变量覆盖配置文件。
func addressFallback(fileVal *bool) *bool {
	v := confrepo.ResolveBool(nil, confrepo.EnvNames("ADDRESS_FALLBACK"), fileVal, true)
	return &v
}
