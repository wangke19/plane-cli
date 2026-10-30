package app

import (
	"fmt"
	"net/http"

	"plane-cli/internal/api"
	"plane-cli/internal/textutil"
)

// ---- cycle ----

func cmdCycle(a *App, argv []string) error {
	if len(argv) == 0 {
		return usagef("cycle 需要 子命令: list | create | issues | add")
	}
	sub, rest := argv[0], argv[1:]
	switch sub {
	case "list":
		return cycleList(a, rest)
	case "create":
		return cycleCreate(a, rest)
	case "issues":
		return cycleIssues(a, rest)
	case "add":
		return cycleAdd(a, rest)
	default:
		return usagef("未知 cycle 子命令 '%s'", sub)
	}
}

func cycleList(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.booleanAlias("h", "help")
	f.boolean("json")
	if _, err := f.parse(argv); err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane cycle list [-p 项目] [--json]")
		return nil
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	cycles, err := a.Client.Cycles(p.ID)
	if err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(cycles)
		return nil
	}
	if len(cycles) == 0 {
		fmt.Fprintln(a.Out, "（无迭代周期）")
		return nil
	}
	rows := make([][]string, len(cycles))
	for i, c := range cycles {
		rows[i] = []string{c.Name, textutil.ShortDate(c.StartDate), textutil.ShortDate(c.EndDate)}
	}
	printTable(a.Out, []string{"名称", "开始", "结束"}, rows)
	return nil
}

func cycleCreate(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.booleanAlias("h", "help")
	f.boolean("json")
	f.strAlias("n", "name")
	f.str("start")
	f.str("end")
	f.strAlias("d", "description")
	if _, err := f.parse(argv); err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane cycle create -n <名称> --start YYYY-MM-DD --end YYYY-MM-DD [-d 描述] [-p 项目]")
		return nil
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	name := f.val("n", "")
	if name == "" {
		return usagef("缺少 -n <名称>")
	}
	start, err := checkDate(f.val("start", ""))
	if err != nil {
		return err
	}
	end, err := checkDate(f.val("end", ""))
	if err != nil {
		return err
	}
	payload := map[string]any{"name": name, "start_date": start, "end_date": end}
	if d := f.val("d", ""); d != "" {
		payload["description"] = d
	}
	resp, err := a.Client.Do(http.MethodPost,
		fmt.Sprintf("workspaces/%s/projects/%s/cycles/", a.Cfg.Workspace, p.ID), payload)
	if err != nil {
		return err
	}
	c := &api.Cycle{}
	if err := resp.Decode(c); err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(c)
		return nil
	}
	fmt.Fprintf(a.Out, "✅ 已创建周期 %s  (%s ~ %s)\n", c.Name, textutil.ShortDate(c.StartDate), textutil.ShortDate(c.EndDate))
	return nil
}

func cycleIssues(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.booleanAlias("h", "help")
	f.boolean("json")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane cycle issues <周期> [-p 项目] [--json]")
		return nil
	}
	if len(args) < 1 {
		return usagef("缺少周期（名称或 id 前缀）")
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	c, err := a.Client.ResolveCycle(p.ID, args[0])
	if err != nil {
		return err
	}
	items, err := a.Client.CycleIssues(p.ID, c.ID)
	if err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(items)
		return nil
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{refID(p, &it), priorityOrNone(it.Priority),
			textutil.ShortDate(it.TargetDate), textutil.Clip(textutil.HTMLToText(it.Name), 46)}
	}
	printTable(a.Out, []string{"编号", "优先级", "截止", "标题"}, rows)
	fmt.Fprintf(a.Out, "\n周期 %s  共 %d 条任务\n", c.Name, len(items))
	return nil
}

func cycleAdd(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.booleanAlias("h", "help")
	f.boolean("remove")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane cycle add <周期> <编号> [-p 项目] [--remove]")
		return nil
	}
	if len(args) < 2 {
		return usagef("用法: plane cycle add <周期> <编号>")
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	c, err := a.Client.ResolveCycle(p.ID, args[0])
	if err != nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, args[1])
	if err != nil {
		return err
	}
	base := fmt.Sprintf("workspaces/%s/projects/%s/cycles/%s/cycle-issues/", a.Cfg.Workspace, p.ID, c.ID)
	if f.isSet("remove") {
		if _, err := a.Client.Do(http.MethodDelete, base+it.ID+"/", nil,
			http.StatusOK, http.StatusNoContent); err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "✅ 已把 %s 移出周期 %s\n", refID(p, it), c.Name)
		return nil
	}
	if _, err := a.Client.Do(http.MethodPost, base, map[string]any{"issues": []string{it.ID}}); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "✅ 已把 %s 加入周期 %s\n", refID(p, it), c.Name)
	return nil
}

// ---- module ----

func cmdModule(a *App, argv []string) error {
	if len(argv) == 0 {
		return usagef("module 需要 子命令: list | create")
	}
	sub, rest := argv[0], argv[1:]
	switch sub {
	case "list":
		f := addProjectFlag(newFlagSet())
		f.booleanAlias("h", "help")
		f.boolean("json")
		if _, err := f.parse(rest); err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, "用法: plane module list [-p 项目] [--json]")
			return nil
		}
		p, err := a.resolveProject(f.val("p", ""))
		if err != nil {
			return err
		}
		modules, err := a.Client.Modules(p.ID)
		if err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(modules)
			return nil
		}
		if len(modules) == 0 {
			fmt.Fprintln(a.Out, "（无模块）")
			return nil
		}
		rows := make([][]string, len(modules))
		for i, m := range modules {
			rows[i] = []string{m.Name, textutil.ShortDate(m.StartDate), textutil.ShortDate(m.TargetDate), shortID(m.ID)}
		}
		printTable(a.Out, []string{"名称", "开始", "目标", "ID"}, rows)
		return nil

	case "create":
		f := addProjectFlag(newFlagSet())
		f.booleanAlias("h", "help")
		f.boolean("json")
		f.strAlias("n", "name")
		f.strAlias("d", "description")
		f.str("start")
		f.str("target")
		if _, err := f.parse(rest); err != nil {
			return err
		}
		if f.helpReq {
			fmt.Fprintln(a.Out, "用法: plane module create -n <名称> [-d 描述] [--start YYYY-MM-DD] [--target YYYY-MM-DD] [-p 项目]")
			return nil
		}
		p, err := a.resolveProject(f.val("p", ""))
		if err != nil {
			return err
		}
		name := f.val("n", "")
		if name == "" {
			return usagef("缺少 -n <名称>")
		}
		payload := map[string]any{"name": name}
		if d := f.val("d", ""); d != "" {
			payload["description"] = d
		}
		if v := f.val("start", ""); v != "" {
			if payload["start_date"], err = checkDate(v); err != nil {
				return err
			}
		}
		if v := f.val("target", ""); v != "" {
			if payload["target_date"], err = checkDate(v); err != nil {
				return err
			}
		}
		resp, err := a.Client.Do(http.MethodPost,
			fmt.Sprintf("workspaces/%s/projects/%s/modules/", a.Cfg.Workspace, p.ID), payload)
		if err != nil {
			return err
		}
		m := &api.Module{}
		if err := resp.Decode(m); err != nil {
			return err
		}
		if f.isSet("json") {
			a.printJSON(m)
			return nil
		}
		fmt.Fprintf(a.Out, "✅ 已创建模块 %s\n", m.Name)
		return nil

	default:
		return usagef("未知 module 子命令 '%s'", sub)
	}
}
