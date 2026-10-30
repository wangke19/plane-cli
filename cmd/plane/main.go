// plane 命令行入口。
package main

import "os"

// version 由构建注入：-ldflags "-X main.version=..."
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	return runWith(argv, version, os.Stdout, os.Stderr)
}
