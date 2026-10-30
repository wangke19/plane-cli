package app

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"plane-cli/internal/api"
	"plane-cli/internal/textutil"
)

// ---- auth ----

func cmdAuth(a *App, argv []string) error {
	f := newFlagSet()
	f.booleanAlias("h", "help")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane auth status   # 检查 token 与当前用户")
		return nil
	}
	if len(args) > 0 && args[0] != "status" {
		return usagef("auth 只支持 status 子命令，得到 '%s'", strings.Join(args, " "))
	}
	me, err := a.Client.Me()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(me.FirstName + " " + me.LastName)
	if name == "" {
		name = me.Email
	}
	fmt.Fprintf(a.Out, "✅ 认证有效: %s <%s>\n", name, me.Email)
	fmt.Fprintf(a.Out, "   服务端: %s   工作区: %s   用户id: %s\n", a.Cfg.BaseURL, a.Cfg.Workspace, shortID(me.ID))
	return nil
}

// ---- project ----

// forbidChars 项目名中 Plane 不允许出现的字符（含连字符）。
var forbidChars = regexp.MustCompile(`[-._!@#$%^&*()+=[\]{};:'"\\|,<>/?~` + "`" + `]`)

func cmdProject(a *App, argv []string) error {
	if len(argv) == 0 {
		return usagef("project 需要 子命令: list | view | create")
	}
	sub, rest := argv[0], argv[1:]
	switch sub {
	case "list":
		f := newFlagSet()
		f.booleanAlias("h", "help")
		f.boolean("json")
		if _, err := f.parse(rest); err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, "用法: plane project list [--json]")
			return nil
		}
		projects, err := a.Client.Projects()
		if err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(projects)
			return nil
		}
		rows := make([][]string, len(projects))
		for i, p := range projects {
			rows[i] = []string{p.Identifier, textutil.Clip(p.Name, 24), textutil.ShortDate(p.CreatedAt), shortID(p.ID)}
		}
		printTable(a.Out, []string{"标识符", "名称", "创建日期", "ID"}, rows)
		fmt.Fprintf(a.Out, "\n共 %d 个项目\n", len(projects))
		return nil

	case "view":
		f := newFlagSet()
		f.booleanAlias("h", "help")
		f.boolean("json")
		args, err := f.parse(rest)
		if err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, "用法: plane project view <项目>")
			return nil
		}
		key, err := f.require(args, "项目")
		if err != nil {
			return err
		}
		p, err := a.Client.ResolveProject(key)
		if err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(p)
			return nil
		}
		fmt.Fprintf(a.Out, "%s · %s  (id=%s)\n", p.Identifier, p.Name, shortID(p.ID))
		if p.Description != "" {
			fmt.Fprintf(a.Out, "\n%s\n", textutil.HTMLToText(p.Description))
		}
		issues, err := a.Client.Issues(p.ID)
		if err != nil {
			return err
		}
		counts := map[string]int{}
		for _, it := range issues {
			if st, err := a.Client.ResolveState(p.ID, it.State); err == nil {
				counts[st.Group]++
			} else {
				counts["?"]++
			}
		}
		fmt.Fprint(a.Out, "\n任务统计:")
		for _, g := range []string{"backlog", "unstarted", "started", "completed", "cancelled"} {
			fmt.Fprintf(a.Out, "  %s=%d", g, counts[g])
		}
		fmt.Fprintf(a.Out, "\n共 %d 条任务  |  %s/%s/projects/%s/issues/\n",
			len(issues), a.Cfg.BaseURL, a.Cfg.Workspace, p.ID)
		return nil

	case "create":
		f := newFlagSet()
		f.booleanAlias("h", "help")
		f.boolean("json")
		f.boolean("private")
		f.strAlias("d", "description")
		f.str("identifier")
		args, err := f.parse(rest)
		if err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, `用法: plane project create <名称> [-d 描述] [--identifier 标识符] [--private]
注意：名称不能含连字符等特殊字符（空格/中文/字母数字均可）`)
			return nil
		}
		if len(args) < 1 {
			return usagef("缺少项目名称")
		}
		name := args[0]
		if m := forbidChars.FindString(name); m != "" {
			return failf("项目名含 Plane 禁止的字符 %q（连字符也不行），请改用空格或中文", m)
		}
		ident := f.val("identifier", "")
		if ident == "" {
			ident = strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(name, ""))
			if len(ident) > 12 {
				ident = ident[:12]
			}
		}
		if ident == "" {
			return failf("无法从项目名生成标识符，请用 --identifier 指定（纯大写字母数字）")
		}
		network := 2
		if f.isSet("private") {
			network = 0
		}
		payload := map[string]any{
			"name": name, "identifier": ident,
			"description": f.val("d", ""), "network": network,
		}
		resp, err := a.Client.Do(http.MethodPost, fmt.Sprintf("workspaces/%s/projects/", a.Cfg.Workspace), payload)
		if err != nil {
			return err
		}
		p := &api.Project{}
		if err := resp.Decode(p); err != nil {
			return err
		}
		// API 建的项目默认关闭周期/模块视图，补开与网页端一致
		if _, err := a.Client.Do(http.MethodPatch,
			fmt.Sprintf("workspaces/%s/projects/%s/", a.Cfg.Workspace, p.ID),
			map[string]any{"cycle_view": true, "module_view": true}); err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(p)
			return nil
		}
		fmt.Fprintf(a.Out, "✅ 已创建项目 %s · %s  (id=%s)\n", p.Identifier, p.Name, shortID(p.ID))
		fmt.Fprintf(a.Out, "   %s/%s/projects/%s/issues/\n", a.Cfg.BaseURL, a.Cfg.Workspace, p.ID)
		return nil

	default:
		return usagef("未知 project 子命令 '%s'（list | view | create）", sub)
	}
}

// shortID 取 id 前 8 位。
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
