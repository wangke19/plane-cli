package app

import (
	"errors"
	"fmt"
	"io"

	"plane-cli/internal/api"
	"plane-cli/internal/config"
)

const usageText = `plane — gh 风格的 Plane 项目管理命令行工具

用法：
  plane <命令> [子命令] [参数]

命令：
  auth status                          检查 token 与当前用户
  project list|view|create             项目操作
  issue list|view|create|edit|close|reopen|delete|comment|comments|assign|label
                                       任务操作
  state list                           状态列表
  label list|create                    标签操作
  cycle list|create|issues|add         周期/迭代操作
  module list|create                   模块操作
  api METHOD PATH [JSON]               通用 API 调用（逃生舱）

配置（环境变量）：
  PLANE_TOKEN       API Token（必填）
  PLANE_BASE_URL    服务地址（必填）
  PLANE_WORKSPACE   工作区 slug（必填）
  PLANE_PROJECT     默认项目（可选）

全部列表命令支持 --json 输出原始 JSON。每个命令支持 -h 查看参数。

手册：https://github.com/plane-cli/plane-cli#readme
`

// Run 执行一次 CLI 调用，返回进程退出码（0 成功 / 1 失败 / 2 用法错误）。
func Run(argv []string, version string, out, errOut io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprint(errOut, usageText)
		return 2
	}
	switch argv[0] {
	case "--version", "-V", "version":
		fmt.Fprintf(out, "plane %s\n", version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(out, usageText)
		return 0
	}

	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(errOut, "plane: 错误: %v\n", err)
		return 1
	}
	a := &App{
		Out:    out,
		ErrOut: errOut,
		Cfg:    cfg,
		Client: api.New(cfg.BaseURL, cfg.Token, cfg.Workspace),
	}

	var runErr error
	switch argv[0] {
	case "auth":
		runErr = cmdAuth(a, argv[1:])
	case "project":
		runErr = cmdProject(a, argv[1:])
	case "issue":
		runErr = cmdIssue(a, argv[1:])
	case "state":
		runErr = cmdState(a, argv[1:])
	case "label":
		runErr = cmdLabel(a, argv[1:])
	case "cycle":
		runErr = cmdCycle(a, argv[1:])
	case "module":
		runErr = cmdModule(a, argv[1:])
	case "api":
		runErr = cmdAPI(a, argv[1:])
	default:
		fmt.Fprintf(errOut, "plane: 未知命令 '%s'（--help 查看用法）\n", argv[0])
		return 2
	}
	if runErr != nil {
		code := 1
		var ee *ExitError
		if errors.As(runErr, &ee) {
			code = ee.Code
		}
		fmt.Fprintf(errOut, "plane: 错误: %v\n", runErr)
		return code
	}
	return 0
}
