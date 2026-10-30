package main

import (
	"io"

	"plane-cli/internal/app"
)

// runWith 便于测试注入输出流。
func runWith(argv []string, version string, out, errOut io.Writer) int {
	return app.Run(argv, version, out, errOut)
}
