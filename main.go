package main

import (
	"os"

	"licode/cmd"
)

func main() {
	// licode 直接运行即启动 Web 服务器。
	if err := cmd.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
