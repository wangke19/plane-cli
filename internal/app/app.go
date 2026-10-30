// Package app 实现 plane CLI 的命令层：参数解析、命令分发与各子命令。
package app

import (
	"encoding/json"
	"fmt"
	"io"

	"plane-cli/internal/api"
	"plane-cli/internal/config"
)

// App 携带一次命令执行所需的全部上下文。
type App struct {
	Out    io.Writer
	ErrOut io.Writer
	Cfg    config.Config
	Client *api.Client
}

// ExitError 携带进程退出码的错误；用于可预期的失败（找不到资源、参数不合法等）。
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }

// failf 构造退出码为 1 的错误。
func failf(format string, a ...any) *ExitError {
	return &ExitError{Code: 1, Msg: fmt.Sprintf(format, a...)}
}

// usagef 构造退出码为 2 的参数用法错误。
func usagef(format string, a ...any) *ExitError {
	return &ExitError{Code: 2, Msg: fmt.Sprintf(format, a...)}
}

// printJSON 以缩进形式输出任意值。
func (a *App) printJSON(v any) {
	enc := json.NewEncoder(a.Out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// projectKey 决定目标项目：显式 -p 优先，其次 PLANE_PROJECT。
func (a *App) projectKey(explicit string) (string, error) {
	if k := explicit; k != "" {
		return k, nil
	}
	if k := a.Cfg.DefaultProject; k != "" {
		return k, nil
	}
	return "", failf("未指定项目：用 -p <项目名/标识符/id> 或设置 PLANE_PROJECT")
}

// resolveProject 在 projectKey 基础上完成到项目对象的解析。
func (a *App) resolveProject(explicit string) (*api.Project, error) {
	key, err := a.projectKey(explicit)
	if err != nil {
		return nil, err
	}
	return a.Client.ResolveProject(key)
}

// MemberNames 把成员 id 列表转为展示名串，空列表返回 "-"。
func (a *App) MemberNames(ids []string) string {
	if len(ids) == 0 {
		return "-"
	}
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += a.Client.MemberName(id)
	}
	return out
}
