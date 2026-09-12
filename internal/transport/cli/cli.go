// Package cli 是命令行适配层：解析参数、调用用例、输出结果。
// 这一层不含业务判断，只做协议翻译。
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

不带 -f 时从标准输入读取。
`

// Run 执行一次命令，返回进程退出码。
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "", "输入文件，缺省从标准输入读取")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	text, err := readInput(*file, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "读取输入失败:", err)
		return 1
	}

	app, err := bootstrap.InitApp(bootstrap.Config{
		ModelsDir: confrepo.Resolve("", confrepo.EnvNames("MODELS_DIR"), "", ""),
	})
	if err != nil {
		fmt.Fprintln(stderr, "初始化失败:", err)
		return 1
	}

	ctx := context.Background()
	switch cmd {
	case "mask":
		masked, _, err := app.Masker.Mask(ctx, text)
		if err != nil {
			fmt.Fprintln(stderr, "脱敏失败:", err)
			return 1
		}
		fmt.Fprint(stdout, masked)
	case "spans":
		spans, err := app.Masker.Spans(ctx, text)
		if err != nil {
			fmt.Fprintln(stderr, "识别失败:", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(spansView(text, spans)); err != nil {
			fmt.Fprintln(stderr, "序列化失败:", err)
			return 1
		}
	case "roundtrip":
		masked, meta, err := app.Masker.Mask(ctx, text)
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
