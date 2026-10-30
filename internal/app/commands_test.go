package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// 命令级功能测试：经由 app.Run 走完整链路（配置→参数解析→API→渲染），
// 断言 stdout 呈现、进程退出码与实际发出的 HTTP 请求体。

func TestAuthStatusCmd(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "auth", "status")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "me@example.com") || !strings.Contains(out, "acme") {
		t.Errorf("输出缺少用户/工作区信息:\n%s", out)
	}
}

func TestProjectListAndViewCmd(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "project", "list")
	if code != 0 || !strings.Contains(out, "DEMO") || !strings.Contains(out, "demo") {
		t.Errorf("project list 异常 code=%d:\n%s", code, out)
	}
	out, _, code = f.run(t, "project", "view", "demo")
	if code != 0 || !strings.Contains(out, "demo project") {
		t.Errorf("project view 应渲染描述:\n%s", out)
	}
}

func TestProjectCreateCmd(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "project", "create", "new app", "-d", "描述")
	if code != 0 || !strings.Contains(out, "NEWAPP") {
		t.Errorf("标识符应从名称生成:\n%s", out)
	}
	// 创建后应自动打开周期/模块视图
	rec := f.findReq(t, "PATCH", "/projects/")
	jsonField(t, rec.Body, "cycle_view", true)
	jsonField(t, rec.Body, "module_view", true)
	// 非法项目名应快速失败
	_, errOut, code := f.run(t, "project", "create", "bad-name")
	if code != 1 || !strings.Contains(errOut, "禁止的字符") {
		t.Errorf("非法名应报错 code=%d err=%s", code, errOut)
	}
}

func TestIssueListFilters(t *testing.T) {
	f := newFakePlane(t)
	cases := []struct {
		name string
		args []string
		want []string // 必须出现的编号
		omit []string // 不能出现的编号
	}{
		{"默认全部", []string{"issue", "list", "-p", "demo"}, []string{"DEMO-1", "DEMO-2"}, nil},
		{"状态过滤", []string{"issue", "list", "-p", "demo", "--state", "todo"}, []string{"DEMO-1"}, []string{"DEMO-2"}},
		{"优先级过滤", []string{"issue", "list", "-p", "demo", "--priority", "medium"}, []string{"DEMO-2"}, []string{"DEMO-1"}},
		{"只看我负责的", []string{"issue", "list", "-p", "demo", "--mine"}, []string{"DEMO-2"}, []string{"DEMO-1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, code := f.run(t, c.args...)
			if code != 0 {
				t.Fatalf("code=%d out=%s", code, out)
			}
			for _, want := range c.want {
				if !strings.Contains(out, want) {
					t.Errorf("应包含 %s:\n%s", want, out)
				}
			}
			for _, omit := range c.omit {
				if strings.Contains(out, omit) {
					t.Errorf("不应包含 %s:\n%s", omit, out)
				}
			}
		})
	}
}

func TestIssueListJSON(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "issue", "list", "-p", "demo", "--json")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("--json 应输出合法 JSON 数组: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("应有 2 条: %d", len(items))
	}
}

func TestIssueViewCmd(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "issue", "view", "1", "-p", "demo")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	for _, want := range []string{"DEMO-1", "Login broken", "cannot sign in", "Alice", "2026-12-01", "high"} {
		if !strings.Contains(out, want) {
			t.Errorf("view 输出缺少 %q:\n%s", want, out)
		}
	}
}

func TestIssueCreateCmd(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "issue", "create", "-p", "demo", "-t", "新任务", "-d", "第一行\n第二行", "--priority", "high", "--due", "2026-12-15")
	if code != 0 || !strings.Contains(out, "DEMO-3") {
		t.Fatalf("新任务编号应为 3:\n%s", out)
	}
	rec := f.findReq(t, "POST", "/issues/")
	jsonField(t, rec.Body, "name", "新任务")
	jsonField(t, rec.Body, "priority", "high")
	jsonField(t, rec.Body, "target_date", "2026-12-15")
	jsonField(t, rec.Body, "state", "st-todo") // 缺省落在 Todo
	var body map[string]any
	_ = json.Unmarshal([]byte(rec.Body), &body)
	if html, _ := body["description_html"].(string); !strings.Contains(html, "第一行<br/>第二行") {
		t.Errorf("描述应转为 HTML 换行: %v", body["description_html"])
	}
}

func TestIssueEditCmd(t *testing.T) {
	f := newFakePlane(t)
	if _, _, code := f.run(t, "issue", "edit", "1", "-p", "demo", "-t", "改标题", "--priority", "low", "-s", "doing"); code != 0 {
		t.Fatal("edit 失败")
	}
	rec := f.findReq(t, "PATCH", "iss-1")
	jsonField(t, rec.Body, "name", "改标题")
	jsonField(t, rec.Body, "priority", "low")
	jsonField(t, rec.Body, "state", "st-doing")
	// 空编辑应报错
	_, errOut, code := f.run(t, "issue", "edit", "1", "-p", "demo")
	if code != 1 || !strings.Contains(errOut, "没有要修改") {
		t.Errorf("空编辑应报错 code=%d err=%s", code, errOut)
	}
}

func TestIssueCloseReopenCmd(t *testing.T) {
	f := newFakePlane(t)
	if _, _, code := f.run(t, "issue", "close", "1", "-p", "demo"); code != 0 {
		t.Fatal("close 失败")
	}
	rec := f.findReq(t, "PATCH", "iss-1")
	jsonField(t, rec.Body, "state", "st-done")
	if _, _, code := f.run(t, "issue", "reopen", "1", "-p", "demo"); code != 0 {
		t.Fatal("reopen 失败")
	}
	rec = f.findReq(t, "PATCH", "iss-1")
	jsonField(t, rec.Body, "state", "st-todo")
}

func TestIssueDeleteCmd(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "issue", "delete", "2", "-p", "demo", "-y")
	if code != 0 || !strings.Contains(out, "已删除") {
		t.Fatalf("code=%d out=%s", code, out)
	}
	f.findReq(t, "DELETE", "iss-2")
	out, _, _ = f.run(t, "issue", "list", "-p", "demo")
	if strings.Contains(out, "DEMO-2") {
		t.Errorf("删除后列表不应再有 DEMO-2:\n%s", out)
	}
}

func TestIssueCommentCmds(t *testing.T) {
	f := newFakePlane(t)
	if _, _, code := f.run(t, "issue", "comment", "1", "-p", "demo", "-d", "已定位原因"); code != 0 {
		t.Fatal("comment 失败")
	}
	rec := f.findReq(t, "POST", "comments")
	jsonField(t, rec.Body, "comment_html", "<p>已定位原因</p>")
	out, _, code := f.run(t, "issue", "comments", "1", "-p", "demo")
	if code != 0 || !strings.Contains(out, "已定位原因") || !strings.Contains(out, "共 1 条评论") {
		t.Errorf("comments 渲染异常:\n%s", out)
	}
}

func TestIssueAssignCmd(t *testing.T) {
	f := newFakePlane(t)
	// 追加指派：现有 alice + me
	if _, _, code := f.run(t, "issue", "assign", "1", "-p", "demo", "me"); code != 0 {
		t.Fatal("assign 失败")
	}
	rec := f.findReq(t, "PATCH", "iss-1")
	jsonField(t, rec.Body, "assignees", []string{"u-alice", "u-me"})
	// 移除
	if _, _, code := f.run(t, "issue", "assign", "1", "-p", "demo", "--remove", "alice"); code != 0 {
		t.Fatal("assign --remove 失败")
	}
	rec = f.findReq(t, "PATCH", "iss-1")
	jsonField(t, rec.Body, "assignees", []string{"u-me"})
	// 清空
	if _, _, code := f.run(t, "issue", "assign", "1", "-p", "demo", "--clear"); code != 0 {
		t.Fatal("assign --clear 失败")
	}
	rec = f.findReq(t, "PATCH", "iss-1")
	jsonField(t, rec.Body, "assignees", []string{})
	// 未知成员报错
	_, errOut, code := f.run(t, "issue", "assign", "1", "-p", "demo", "nobody")
	if code != 1 || !strings.Contains(errOut, "找不到成员") {
		t.Errorf("未知成员应报错 code=%d err=%s", code, errOut)
	}
}

func TestIssueLabelCmd(t *testing.T) {
	f := newFakePlane(t)
	if _, _, code := f.run(t, "label", "create", "perf", "--color", "#00ff00", "-p", "demo"); code != 0 {
		t.Fatal("label create 失败")
	}
	if _, _, code := f.run(t, "issue", "label", "2", "-p", "demo", "perf"); code != 0 {
		t.Fatal("issue label 失败")
	}
	rec := f.findReq(t, "PATCH", "iss-2")
	jsonField(t, rec.Body, "labels", []string{"l-bug", "l-101"})
	// 移除已有标签
	if _, _, code := f.run(t, "issue", "label", "2", "-p", "demo", "--remove", "bug"); code != 0 {
		t.Fatal("issue label --remove 失败")
	}
	rec = f.findReq(t, "PATCH", "iss-2")
	jsonField(t, rec.Body, "labels", []string{"l-101"})
}

func TestStateLabelCycleListCmds(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "state", "list", "-p", "demo")
	if code != 0 || !strings.Contains(out, "Todo") || !strings.Contains(out, "unstarted") {
		t.Errorf("state list 异常:\n%s", out)
	}
	out, _, _ = f.run(t, "label", "list", "-p", "demo")
	if !strings.Contains(out, "bug") || !strings.Contains(out, "#ff0000") {
		t.Errorf("label list 异常:\n%s", out)
	}
	out, _, _ = f.run(t, "cycle", "list", "-p", "demo")
	if !strings.Contains(out, "（无迭代周期）") {
		t.Errorf("空周期应友好提示:\n%s", out)
	}
}

func TestCycleCmds(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "cycle", "create", "-p", "demo", "-n", "Sprint1", "--start", "2026-12-14", "--end", "2026-12-27")
	if code != 0 || !strings.Contains(out, "Sprint1") {
		t.Fatalf("cycle create 异常 code=%d:\n%s", code, out)
	}
	rec := f.findReq(t, "POST", "/cycles/")
	jsonField(t, rec.Body, "name", "Sprint1")
	jsonField(t, rec.Body, "start_date", "2026-12-14")
	// 非法日期
	_, errOut, code := f.run(t, "cycle", "create", "-p", "demo", "-n", "X", "--start", "12/14", "--end", "2026-12-27")
	if code != 1 || !strings.Contains(errOut, "YYYY-MM-DD") {
		t.Errorf("非法日期应报错 code=%d err=%s", code, errOut)
	}
	// 任务进出周期
	if out, errOut, code := f.run(t, "cycle", "add", "Sprint1", "1", "-p", "demo"); code != 0 {
		t.Fatalf("cycle add 失败 code=%d out=%s err=%s", code, out, errOut)
	}
	rec = f.findReq(t, "POST", "cycle-issues")
	jsonField(t, rec.Body, "issues", []string{"iss-1"})
	out, _, _ = f.run(t, "cycle", "issues", "Sprint1", "-p", "demo")
	if !strings.Contains(out, "DEMO-1") {
		t.Errorf("周期内应含 DEMO-1:\n%s", out)
	}
	if _, _, code := f.run(t, "cycle", "add", "Sprint1", "1", "-p", "demo", "--remove"); code != 0 {
		t.Fatal("cycle add --remove 失败")
	}
	f.findReq(t, "DELETE", "cycle-issues/iss-1")
}

func TestModuleCmds(t *testing.T) {
	f := newFakePlane(t)
	out, _, code := f.run(t, "module", "create", "-p", "demo", "-n", "M1")
	if code != 0 || !strings.Contains(out, "M1") {
		t.Fatalf("module create 异常:\n%s", out)
	}
	out, _, _ = f.run(t, "module", "list", "-p", "demo")
	if !strings.Contains(out, "M1") {
		t.Errorf("module list 异常:\n%s", out)
	}
}

func TestAPIEscapeHatchCmd(t *testing.T) {
	f := newFakePlane(t)
	// 非 workspaces 路径需写绝对形式（自动补前缀规则见 api -h）
	out, _, code := f.run(t, "api", "GET", "/api/v1/users/me/")
	if code != 0 || !strings.Contains(out, "me@example.com") {
		t.Errorf("api GET 异常 code=%d:\n%s", code, out)
	}
	// 相对路径自动补 workspaces 前缀
	if out, errOut, code := f.run(t, "api", "GET", "projects/", "--raw"); code != 0 {
		t.Fatalf("api --raw 失败 code=%d out=%s err=%s", code, out, errOut)
	}
	f.findReq(t, "GET", "/projects/")
}

func TestErrorPaths(t *testing.T) {
	f := newFakePlane(t)
	// 未知命令 → 2
	if _, _, code := f.run(t, "nope"); code != 2 {
		t.Errorf("未知命令应返回 2，得到 %d", code)
	}
	// 缺位置参数 → 2
	if _, _, code := f.run(t, "issue", "view"); code != 2 {
		t.Errorf("缺编号应返回 2，得到 %d", code)
	}
	// 资源不存在 → 1
	_, errOut, code := f.run(t, "issue", "view", "999", "-p", "demo")
	if code != 1 || !strings.Contains(errOut, "找不到任务") {
		t.Errorf("不存在任务应返回 1，得到 %d err=%s", code, errOut)
	}
	// 环境缺失 → 1
	_, errOut, code = f.runEnv(t, map[string]string{"PLANE_TOKEN": ""}, "auth", "status")
	if code != 1 || !strings.Contains(errOut, "PLANE_TOKEN") {
		t.Errorf("缺凭据应返回 1，得到 %d err=%s", code, errOut)
	}
}
