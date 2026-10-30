package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"plane-cli/internal/api"
	"plane-cli/internal/textutil"
)

// ---- state ----

func cmdState(a *App, argv []string) error {
	if len(argv) == 0 {
		return usagef("state 需要 子命令: list")
	}
	if argv[0] != "list" {
		return usagef("未知 state 子命令 '%s'", argv[0])
	}
	f := addProjectFlag(newFlagSet())
	f.booleanAlias("h", "help")
	f.boolean("json")
	if _, err := f.parse(argv[1:]); err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane state list [-p 项目] [--json]")
		return nil
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	states, err := a.Client.States(p.ID)
	if err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(states)
		return nil
	}
	rows := make([][]string, len(states))
	for i, s := range states {
		rows[i] = []string{s.Name, s.Group, fmt.Sprintf("%d", s.Sort)}
	}
	printTable(a.Out, []string{"名称", "分组", "排序"}, rows)
	return nil
}

// ---- label ----

func cmdLabel(a *App, argv []string) error {
	if len(argv) == 0 {
		return usagef("label 需要 子命令: list | create")
	}
	sub, rest := argv[0], argv[1:]
	f := addProjectFlag(newFlagSet())
	f.booleanAlias("h", "help")
	f.boolean("json")
	switch sub {
	case "list":
		if _, err := f.parse(rest); err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, "用法: plane label list [-p 项目] [--json]")
			return nil
		}
		p, err := a.resolveProject(f.val("p", ""))
		if err != nil {
			return err
		}
		labels, err := a.Client.Labels(p.ID)
		if err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(labels)
			return nil
		}
		if len(labels) == 0 {
			fmt.Fprintln(a.Out, "（无标签）")
			return nil
		}
		rows := make([][]string, len(labels))
		for i, l := range labels {
			rows[i] = []string{l.Name, l.Color, textutil.Clip(l.Description, 30)}
		}
		printTable(a.Out, []string{"名称", "颜色", "描述"}, rows)
		return nil

	case "create":
		f.str("color")
		f.strAlias("d", "description")
		args, err := f.parse(rest)
		if err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, `用法: plane label create <名称> [--color #ff0000] [-d 描述] [-p 项目]`)
			return nil
		}
		if len(args) < 1 {
			return usagef("缺少标签名称")
		}
		p, err := a.resolveProject(f.val("p", ""))
		if err != nil {
			return err
		}
		payload := map[string]any{"name": args[0], "color": f.val("color", "#3f76ff")}
		if d := f.val("d", ""); d != "" {
			payload["description"] = d
		}
		resp, err := a.Client.Do(http.MethodPost,
			fmt.Sprintf("workspaces/%s/projects/%s/labels/", a.Cfg.Workspace, p.ID), payload)
		if err != nil {
			return err
		}
		l := &api.Label{}
		if err := resp.Decode(l); err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(l)
			return nil
		}
		fmt.Fprintf(a.Out, "✅ 已创建标签 %s (%s)\n", l.Name, l.Color)
		return nil

	default:
		return usagef("未知 label 子命令 '%s'", sub)
	}
}

// ---- api 逃生舱 ----

func cmdAPI(a *App, argv []string) error {
	f := newFlagSet()
	f.booleanAlias("h", "help")
	f.boolean("raw")
	f.strAlias("B", "body-file")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, `用法: plane api <METHOD> <PATH> [JSON体] [-B 文件] [--raw]
PATH 规则：以 / 开头原样使用；以 workspaces/ 开头原样使用；
其余相对路径自动补 workspaces/{工作区}/ 前缀（如 projects/ → workspaces/acme/projects/）`)
		return nil
	}
	if len(args) < 2 {
		return usagef("用法: plane api <METHOD> <PATH> [JSON体]")
	}
	method := strings.ToUpper(args[0])
	switch method {
	case "GET", "POST", "PATCH", "PUT", "DELETE":
	default:
		return usagef("不支持的 METHOD '%s'", args[0])
	}
	path := args[1]
	switch {
	case strings.HasPrefix(path, "/"): // 绝对路径原样使用
	case strings.HasPrefix(path, "workspaces/"): // 已带工作区前缀
	default: // 相对路径自动补 workspaces/{工作区}/
		path = fmt.Sprintf("workspaces/%s/%s", a.Cfg.Workspace, path)
	}
	var body any
	if len(args) >= 3 {
		if err := json.Unmarshal([]byte(strings.Join(args[2:], " ")), &body); err != nil {
			return failf("请求体不是合法 JSON: %v", err)
		}
	} else if path := f.val("B", ""); path != "" {
		raw, err := readTextFile(path)
		if err != nil {
			return err
		}
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &body); err != nil {
				return failf("请求体不是合法 JSON: %v", err)
			}
		}
	}
	resp, err := a.Client.Do(method, path, body,
		http.StatusOK, http.StatusCreated, http.StatusAccepted,
		http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound)
	if err != nil {
		return err
	}
	if f.isSet("raw") {
		fmt.Fprintln(a.Out, string(resp.Raw))
		return nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, resp.Raw, "", "  "); err == nil {
		fmt.Fprintln(a.Out, buf.String())
	} else {
		fmt.Fprintln(a.Out, string(resp.Raw))
	}
	return nil
}
