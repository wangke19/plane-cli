package app

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"plane-cli/internal/api"
	"plane-cli/internal/textutil"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func checkDate(s string) (string, error) {
	if !dateRe.MatchString(s) {
		return "", failf("日期格式应为 YYYY-MM-DD：%s", s)
	}
	return s, nil
}

func refID(p *api.Project, it *api.Issue) string {
	return fmt.Sprintf("%s-%d", p.Identifier, it.SequenceID)
}

func issueURL(a *App, p *api.Project, it *api.Issue) string {
	return fmt.Sprintf("%s/%s/projects/%s/issues/%s", a.Cfg.BaseURL, a.Cfg.Workspace, p.ID, it.ID)
}

// issueStates 拉取项目状态并以 id 索引。
func issueStates(a *App, projectID string) (map[string]api.State, error) {
	states, err := a.Client.States(projectID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]api.State, len(states))
	for _, s := range states {
		m[s.ID] = s
	}
	return m, nil
}

func readTextFile(path string) (string, error) {
	if path == "-" {
		return readAllStdin()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", failf("读取文件 %s: %v", path, err)
	}
	return string(b), nil
}

func cmdIssue(a *App, argv []string) error {
	if len(argv) == 0 {
		return usagef("issue 需要 子命令: list | view | create | edit | close | reopen | delete | comment | comments | assign | label")
	}
	sub, rest := argv[0], argv[1:]
	switch sub {
	case "list":
		return issueList(a, rest)
	case "view":
		return issueView(a, rest)
	case "create":
		return issueCreate(a, rest)
	case "edit":
		return issueEdit(a, rest)
	case "close", "reopen":
		return issueTransition(a, sub, rest)
	case "delete":
		return issueDelete(a, rest)
	case "comment":
		return issueComment(a, rest)
	case "comments":
		return issueComments(a, rest)
	case "assign":
		return issueAssign(a, rest)
	case "label":
		return issueLabelCmd(a, rest)
	default:
		return usagef("未知 issue 子命令 '%s'", sub)
	}
}

// addProjectFlag 声明通用 -p/--project。
func addProjectFlag(f *flagSet) *flagSet {
	f.str("p", "project")
	return f
}

func issueList(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "json", "mine")
	f.str("state", "priority")
	f.str("limit")
	if _, err := f.parse(argv); err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue list [-p 项目] [--state todo|名称] [--priority high] [--mine] [--limit N] [--json]")
		return nil
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	limit := 50
	if v := f.val("limit", ""); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &limit); err != nil || limit <= 0 {
			return usagef("--limit 需要正整数，得到 '%s'", v)
		}
	}
	issues, err := a.Client.Issues(p.ID)
	if err != nil {
		return err
	}
	if v := f.val("state", ""); v != "" {
		target, err := a.Client.ResolveState(p.ID, v)
		if err != nil {
			return err
		}
		issues = filterIssues(issues, func(it api.Issue) bool { return it.State == target.ID })
	}
	if v := f.val("priority", ""); v != "" {
		issues = filterIssues(issues, func(it api.Issue) bool { return it.Priority == v })
	}
	if f.isSet("mine") {
		me, err := a.Client.Me()
		if err != nil {
			return err
		}
		issues = filterIssues(issues, func(it api.Issue) bool {
			for _, id := range it.Assignees {
				if id == me.ID {
					return true
				}
			}
			return false
		})
	}
	if len(issues) > limit {
		issues = issues[:limit]
	}
	if f.isSet("json") {
		a.printJSON(issues)
		return nil
	}
	states, err := issueStates(a, p.ID)
	if err != nil {
		return err
	}
	rows := make([][]string, len(issues))
	for i, it := range issues {
		rows[i] = []string{
			refID(p, &it),
			textutil.Clip(states[it.State].Name, 12),
			priorityOrNone(it.Priority),
			textutil.ShortDate(it.TargetDate),
			textutil.Clip(textutil.HTMLToText(it.Name), 46),
		}
	}
	printTable(a.Out, []string{"编号", "状态", "优先级", "截止", "标题"}, rows)
	fmt.Fprintf(a.Out, "\n共 %d 条  |  项目: %s\n", len(issues), p.Name)
	return nil
}

func filterIssues(issues []api.Issue, keep func(api.Issue) bool) []api.Issue {
	out := issues[:0:0]
	for _, it := range issues {
		if keep(it) {
			out = append(out, it)
		}
	}
	return out
}

func priorityOrNone(p string) string {
	if p == "" {
		return "none"
	}
	return p
}

func issueView(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "json")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue view <编号> [-p 项目] [--json]")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(it)
		return nil
	}
	states, _ := issueStates(a, p.ID)
	st := states[it.State]
	fmt.Fprintf(a.Out, "%s  %s\n", refID(p, it), it.Name)
	fmt.Fprintf(a.Out, "状态: %s(%s)   优先级: %s   截止: %s\n",
		st.Name, st.Group, priorityOrNone(it.Priority), textutil.ShortDate(it.TargetDate))
	fmt.Fprintf(a.Out, "负责人: %s\n", a.MemberNames(it.Assignees))
	fmt.Fprintf(a.Out, "创建: %s   更新: %s\n", textutil.ShortDate(it.CreatedAt), textutil.ShortDate(it.UpdatedAt))
	fmt.Fprintf(a.Out, "链接: %s\n", issueURL(a, p, it))
	if body := textutil.HTMLToText(it.DescriptionHTML); body != "" {
		fmt.Fprintf(a.Out, "\n%s\n%s\n", strings.Repeat("─", 40), body)
	}
	return nil
}

// parseIssueRef 解析 [-p 项目] <编号> 公共结构。
func parseIssueRef(a *App, f *flagSet, args []string) (*api.Project, string, error) {
	if f.helpReq {
		return nil, "", nil
	}
	if len(args) < 1 {
		return nil, "", usagef("缺少任务编号")
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return nil, "", err
	}
	return p, args[0], nil
}

func issueCreate(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "json")
	f.str("t", "title")
	f.str("d", "description")
	f.str("D", "desc-file")
	f.str("priority")
	f.str("state")
	f.str("due")
	if _, err := f.parse(argv); err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, `用法: plane issue create -t <标题> [-d 描述 | -D 文件] [--priority high] [--state todo] [--due YYYY-MM-DD] [-p 项目]`)
		return nil
	}
	title := f.val("t", "")
	if title == "" {
		return usagef("缺少 -t <标题>")
	}
	p, err := a.resolveProject(f.val("p", ""))
	if err != nil {
		return err
	}
	desc := f.val("d", "")
	if path := f.val("D", ""); path != "" {
		if desc, err = readTextFile(path); err != nil {
			return err
		}
	}
	payload := map[string]any{
		"name":             title,
		"description_html": textutil.TextToHTML(desc),
		"priority":         f.val("priority", "none"),
	}
	if s := f.val("state", ""); s != "" {
		st, err := a.Client.ResolveState(p.ID, s)
		if err != nil {
			return err
		}
		payload["state"] = st.ID
	} else if st, err := a.Client.StateByGroup(p.ID, "unstarted"); err == nil {
		payload["state"] = st.ID
	}
	if due := f.val("due", ""); due != "" {
		d, err := checkDate(due)
		if err != nil {
			return err
		}
		payload["target_date"] = d
	}
	resp, err := a.Client.Do(http.MethodPost,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/", a.Cfg.Workspace, p.ID), payload)
	if err != nil {
		return err
	}
	it := &api.Issue{}
	if err := resp.Decode(it); err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(it)
		return nil
	}
	fmt.Fprintf(a.Out, "✅ 已创建 %s: %s\n   %s\n", refID(p, it), title, issueURL(a, p, it))
	return nil
}

func issueEdit(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "json")
	f.str("t", "title")
	f.str("d", "description")
	f.str("D", "desc-file")
	f.str("s", "state")
	f.str("priority")
	f.str("due")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue edit <编号> [-t 标题] [-d 描述|-D 文件] [-s 状态] [--priority high] [--due YYYY-MM-DD] [-p 项目]")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if v := f.val("t", ""); v != "" {
		payload["name"] = v
	}
	if s := f.val("s", ""); s != "" {
		st, err := a.Client.ResolveState(p.ID, s)
		if err != nil {
			return err
		}
		payload["state"] = st.ID
	}
	if v := f.val("priority", ""); v != "" {
		payload["priority"] = v
	}
	if v := f.val("due", ""); v != "" {
		d, err := checkDate(v)
		if err != nil {
			return err
		}
		payload["target_date"] = d
	}
	desc := f.val("d", "")
	if path := f.val("D", ""); path != "" {
		if desc, err = readTextFile(path); err != nil {
			return err
		}
		if desc != "" {
			payload["description_html"] = textutil.TextToHTML(desc)
		}
	} else if desc != "" {
		payload["description_html"] = textutil.TextToHTML(desc)
	}
	if len(payload) == 0 {
		return failf("没有要修改的内容（-t/-d/-D/-s/--priority/--due）")
	}
	resp, err := a.Client.Do(http.MethodPatch,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/%s/", a.Cfg.Workspace, p.ID, it.ID), payload)
	if err != nil {
		return err
	}
	updated := &api.Issue{}
	_ = resp.Decode(updated)
	if f.isSet("json") {
		a.printJSON(updated)
		return nil
	}
	name := updated.Name
	if name == "" {
		name = it.Name
	}
	fmt.Fprintf(a.Out, "✅ 已更新 %s: %s\n", refID(p, it), name)
	return nil
}

func issueTransition(a *App, action string, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintf(a.Out, "用法: plane issue %s <编号> [-p 项目]\n", action)
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	group := "completed"
	verb := "关闭"
	if action == "reopen" {
		group, verb = "unstarted", "重开"
	}
	st, err := a.Client.StateByGroup(p.ID, group)
	if err != nil {
		return failf("项目 %s 没有 group=%s 的状态，无法%s", p.Name, group, verb)
	}
	if _, err := a.Client.Do(http.MethodPatch,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/%s/", a.Cfg.Workspace, p.ID, it.ID),
		map[string]any{"state": st.ID}); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "✅ 已%s %s → %s\n", verb, refID(p, it), st.Name)
	return nil
}

func issueDelete(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "y", "yes")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue delete <编号> [-p 项目] [-y]")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	if !f.isSet("y") && !confirm(fmt.Sprintf("确认删除 %s 「%s」? [y/N] ", refID(p, it), it.Name)) {
		fmt.Fprintln(a.Out, "已取消")
		return nil
	}
	if _, err := a.Client.Do(http.MethodDelete,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/%s/", a.Cfg.Workspace, p.ID, it.ID),
		nil, http.StatusOK, http.StatusNoContent); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "✅ 已删除 %s\n", refID(p, it))
	return nil
}

func issueComment(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help")
	f.str("d", "text")
	f.str("F", "text-file")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue comment <编号> [-p 项目] -d <内容> | -F <文件(-为stdin)>")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	text := f.val("d", "")
	if path := f.val("F", ""); path != "" {
		if text, err = readTextFile(path); err != nil {
			return err
		}
	}
	if strings.TrimSpace(text) == "" {
		return failf("评论内容为空：用 -d <内容> 或 -F <文件>（- 为 stdin）")
	}
	if _, err := a.Client.Do(http.MethodPost,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/%s/comments/", a.Cfg.Workspace, p.ID, it.ID),
		map[string]any{"comment_html": textutil.TextToHTML(text)}); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "✅ 已评论 %s\n", refID(p, it))
	return nil
}

func issueComments(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "json")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue comments <编号> [-p 项目] [--json]")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	comments, err := a.Client.Comments(p.ID, it.ID)
	if err != nil {
		return err
	}
	if f.isSet("json") {
		a.printJSON(comments)
		return nil
	}
	for _, c := range comments {
		who := "-"
		if c.Actor != "" {
			who = a.Client.MemberName(c.Actor)
		}
		fmt.Fprintf(a.Out, "── %s %s\n   %s\n", textutil.ShortDate(c.CreatedAt), who,
			strings.ReplaceAll(textutil.HTMLToText(c.CommentHTML), "\n", "\n   "))
	}
	if len(comments) == 0 {
		fmt.Fprintln(a.Out, "（暂无评论）")
	} else {
		fmt.Fprintf(a.Out, "\n共 %d 条评论\n", len(comments))
	}
	return nil
}

func issueAssign(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "clear")
	f.list("remove")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue assign <编号> [成员...] [-p 项目] [--remove 成员...] [--clear]")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	members := args[1:]
	if len(members) == 0 && len(f.values("remove")) == 0 && !f.isSet("clear") {
		fmt.Fprintf(a.Out, "%s 负责人: %s\n", refID(p, it), a.MemberNames(it.Assignees))
		return nil
	}
	addIDs, err := resolveMembers(a, members)
	if err != nil {
		return err
	}
	removeIDs, err := resolveMembers(a, f.values("remove"))
	if err != nil {
		return err
	}
	final := mergeIDs(it.Assignees, addIDs, removeIDs, f.isSet("clear"))
	if err := patchIssueIDs(a, p.ID, it.ID, "assignees", final); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "✅ %s 负责人: %s\n", refID(p, it), a.MemberNames(final))
	return nil
}

func issueLabelCmd(a *App, argv []string) error {
	f := addProjectFlag(newFlagSet())
	f.boolean("h", "help", "clear")
	f.list("remove")
	args, err := f.parse(argv)
	if err != nil {
		return err
	}
	if f.helpReq {
		fmt.Fprintln(a.Out, "用法: plane issue label <编号> [标签...] [-p 项目] [--remove 标签...] [--clear]")
		return nil
	}
	p, ref, err := parseIssueRef(a, f, args)
	if err != nil || p == nil {
		return err
	}
	it, err := a.Client.ResolveIssue(p.ID, ref)
	if err != nil {
		return err
	}
	labels := args[1:]
	if len(labels) == 0 && len(f.values("remove")) == 0 && !f.isSet("clear") {
		fmt.Fprintf(a.Out, "%s 标签: %s\n", refID(p, it), labelNames(a, p.ID, it.Labels))
		return nil
	}
	addIDs := make([]string, 0, len(labels))
	for _, n := range labels {
		l, err := a.Client.ResolveLabel(p.ID, n)
		if err != nil {
			return err
		}
		addIDs = append(addIDs, l.ID)
	}
	removeIDs := make([]string, 0, len(f.values("remove")))
	for _, n := range f.values("remove") {
		l, err := a.Client.ResolveLabel(p.ID, n)
		if err != nil {
			return err
		}
		removeIDs = append(removeIDs, l.ID)
	}
	final := mergeIDs(it.Labels, addIDs, removeIDs, f.isSet("clear"))
	if err := patchIssueIDs(a, p.ID, it.ID, "labels", final); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "✅ %s 标签: %s\n", refID(p, it), labelNames(a, p.ID, final))
	return nil
}

func labelNames(a *App, projectID string, ids []string) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if l, err := a.Client.ResolveLabel(projectID, id); err == nil {
			out = append(out, l.Name)
		} else {
			out = append(out, shortID(id))
		}
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

func resolveMembers(a *App, keys []string) ([]string, error) {
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		id, err := a.Client.ResolveMember(k)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// mergeIDs 合并 add/remove/clear 语义，返回去重后的最终列表。
func mergeIDs(current, add, remove []string, clear bool) []string {
	removed := map[string]bool{}
	for _, id := range remove {
		removed[id] = true
	}
	seen := map[string]bool{}
	result := []string{}
	if !clear {
		for _, id := range current {
			if !removed[id] && !seen[id] {
				result = append(result, id)
				seen[id] = true
			}
		}
	}
	for _, id := range add {
		if !removed[id] && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result
}

func patchIssueIDs(a *App, projectID, issueID, field string, ids []string) error {
	_, err := a.Client.Do(http.MethodPatch,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/%s/", a.Cfg.Workspace, projectID, issueID),
		map[string]any{field: ids})
	return err
}
