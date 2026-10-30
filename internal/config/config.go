// Package config 负责加载 CLI 配置。
//
// 所有配置均来自环境变量（安全、可移植、不落盘）：
//
//	PLANE_TOKEN      API Token（必填）
//	PLANE_BASE_URL   服务地址，如 https://plane.example.com（必填）
//	PLANE_WORKSPACE  工作区 slug（必填）
//	PLANE_PROJECT    默认项目（可选；设置后命令里的 -p 可省略）
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config 是一次 CLI 运行所需的全部连接信息。
type Config struct {
	Token          string
	BaseURL        string
	Workspace      string
	DefaultProject string
}

// FromEnv 从环境变量构建配置，缺少必填项时返回包含全部缺失项的错误。
func FromEnv() (Config, error) {
	c := Config{
		Token:          strings.TrimSpace(os.Getenv("PLANE_TOKEN")),
		BaseURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("PLANE_BASE_URL")), "/"),
		Workspace:      strings.TrimSpace(os.Getenv("PLANE_WORKSPACE")),
		DefaultProject: strings.TrimSpace(os.Getenv("PLANE_PROJECT")),
	}
	var missing []string
	if c.Token == "" {
		missing = append(missing, "PLANE_TOKEN")
	}
	if c.BaseURL == "" {
		missing = append(missing, "PLANE_BASE_URL")
	}
	if c.Workspace == "" {
		missing = append(missing, "PLANE_WORKSPACE")
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("缺少环境变量: %s", strings.Join(missing, ", "))
	}
	return c, nil
}
