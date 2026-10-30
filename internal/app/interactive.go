package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// stdinReader 惰性创建的 stdin 读取器（交互确认与 -F - 复用同一个）。
var stdinReader *bufio.Reader

func stdIn() *bufio.Reader {
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	return stdinReader
}

// readAllStdin 读取全部标准输入（-F - / -D - 时用）。
func readAllStdin() (string, error) {
	b, err := io.ReadAll(stdIn())
	if err != nil {
		return "", failf("读取 stdin: %v", err)
	}
	return string(b), nil
}

// confirm 打印提示并读取 y/n；非交互（EOF）时视为否。
func confirm(prompt string) bool {
	fmt.Print(prompt)
	line, err := stdIn().ReadString('\n')
	if err != nil && line == "" {
		fmt.Println()
		return false
	}
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}
