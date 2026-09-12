// Command cloak 是 OneAIFW Go 版的命令行入口。
package main

import (
	"os"

	"github.com/openmodu/cloak/internal/transport/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
