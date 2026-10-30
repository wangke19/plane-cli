package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakePlane 是一个进程内的最小 Plane v1 API 模拟器：
// 覆盖 CLI 用到的全部端点，带内存状态（建任务/改状态/周期成员真实生效），
// 并记录每个请求（方法/路径/请求体）供测试断言。
type fakePlane struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	requests []reqRec

	me      map[string]any
	members []map[string]any

	projects   []*fakeProject
	states     []*fakeState
	issues     []*fakeIssue
	labels     []*fakeLabel
	cycles     []*fakeCycle
	modules    []*fakeModule
	cycleItems map[string][]string         // cycleID -> issueIDs
	comments   map[string][]map[string]any // issueID -> comments

	nextIssueSeq int
	nextID       int
}

type reqRec struct {
	Method string
	Path   string
	Body   string
}

type fakeProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
}

type fakeState struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

type fakeIssue struct {
	ID         string   `json:"id"`
	Project    string   `json:"project"`
	SequenceID int      `json:"sequence_id"`
	Name       string   `json:"name"`
	DescHTML   string   `json:"description_html"`
	Priority   string   `json:"priority"`
	State      string   `json:"state"`
	TargetDate string   `json:"target_date"`
	Assignees  []string `json:"assignees"`
	Labels     []string `json:"labels"`
}

type fakeLabel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type fakeCycle struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type fakeModule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// newFakePlane 启动模拟服务器并注册清理；种子数据：项目 demo、两个任务、bug 标签、五个状态。
func newFakePlane(t *testing.T) *fakePlane {
	t.Helper()
	f := &fakePlane{
		me: map[string]any{"id": "u-me", "email": "me@example.com", "first_name": "Test", "last_name": "User"},
		members: []map[string]any{
			{"id": "u-me", "email": "me@example.com", "first_name": "Test", "last_name": "User"},
			{"id": "u-alice", "email": "alice@example.com", "first_name": "Alice", "last_name": ""},
			{"id": "u-bot", "email": "bot@example.com", "first_name": "", "last_name": "Bot"},
		},
		projects: []*fakeProject{{ID: "p-demo", Name: "demo", Identifier: "DEMO", Description: "<p>demo project</p>"}},
		states: []*fakeState{
			{ID: "st-backlog", Name: "Backlog", Group: "backlog"},
			{ID: "st-todo", Name: "Todo", Group: "unstarted"},
			{ID: "st-doing", Name: "In Progress", Group: "started"},
			{ID: "st-done", Name: "Done", Group: "completed"},
			{ID: "st-cancel", Name: "Cancelled", Group: "cancelled"},
		},
		cycleItems:   map[string][]string{},
		comments:     map[string][]map[string]any{},
		nextIssueSeq: 2,
		nextID:       100,
	}
	f.issues = []*fakeIssue{
		{ID: "iss-1", Project: "p-demo", SequenceID: 1, Name: "Login broken",
			DescHTML: "<p>cannot sign in</p>", Priority: "high", State: "st-todo",
			TargetDate: "2026-12-01", Assignees: []string{"u-alice"}},
		{ID: "iss-2", Project: "p-demo", SequenceID: 2, Name: "Add dark mode",
			Priority: "medium", State: "st-backlog", Assignees: []string{"u-me"},
			Labels: []string{"l-bug"}},
	}
	f.labels = []*fakeLabel{{ID: "l-bug", Name: "bug", Color: "#ff0000"}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// run 用模拟服务器跑一条完整命令，返回 stdout/stderr/退出码。
func (f *fakePlane) run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	return f.runEnv(t, nil, args...)
}

// runEnv 同 run，但允许覆盖环境变量（设为空字符串表示清除）。
func (f *fakePlane) runEnv(t *testing.T, env map[string]string, args ...string) (string, string, int) {
	t.Helper()
	t.Setenv("PLANE_TOKEN", "test-token")
	t.Setenv("PLANE_BASE_URL", f.srv.URL)
	t.Setenv("PLANE_WORKSPACE", "acme")
	t.Setenv("PLANE_PROJECT", "")
	for k, v := range env {
		t.Setenv(k, v)
	}
	var out, errb bytes.Buffer
	code := Run(args, "test", &out, &errb)
	return out.String(), errb.String(), code
}

func (f *fakePlane) record(r *http.Request, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, reqRec{Method: r.Method, Path: r.URL.Path, Body: body})
}

// findReq 返回最后一条匹配（方法+路径子串）的请求；没找到则使测试失败。
func (f *fakePlane) findReq(t *testing.T, method, pathSub string) reqRec {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.requests) - 1; i >= 0; i-- {
		if f.requests[i].Method == method && strings.Contains(f.requests[i].Path, pathSub) {
			return f.requests[i]
		}
	}
	t.Fatalf("未找到请求 %s %s；全部请求: %+v", method, pathSub, f.requests)
	return reqRec{}
}

// jsonField 断言请求体 JSON 中指定字段等于期望值。
func jsonField(t *testing.T, body, field string, want any) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("请求体不是 JSON: %v\n%s", err, body)
	}
	got, ok := m[field]
	if !ok {
		t.Fatalf("请求体缺少字段 %s: %s", field, body)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("字段 %s = %s, 期望 %s\n请求体: %s", field, gotJSON, wantJSON, body)
	}
}

// ---- HTTP 路由与处理 ----

func (f *fakePlane) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.record(r, string(body))
	r.Body = io.NopCloser(bytes.NewReader(body)) // 供下游重新读取
	if r.Header.Get("X-API-Key") != "test-token" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"no auth"}`))
		return
	}
	seg := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// api / v1 / workspaces / acme / ...
	if len(seg) >= 4 && seg[0] == "api" && seg[2] == "workspaces" {
		f.serveWS(w, r, seg[4:])
		return
	}
	if r.URL.Path == "/api/v1/users/me/" {
		writeJSON(w, f.me)
		return
	}
	http.NotFound(w, r)
}

func (f *fakePlane) serveWS(w http.ResponseWriter, r *http.Request, seg []string) {
	switch {
	case len(seg) == 1 && seg[0] == "members":
		writeJSON(w, f.members)
	case len(seg) == 1 && seg[0] == "projects" && r.Method == http.MethodGet:
		writeJSON(w, map[string]any{"results": f.projects, "next_cursor": ""})
	case len(seg) == 1 && seg[0] == "projects" && r.Method == http.MethodPost:
		var body map[string]any
		decode(r, &body)
		f.nextID++
		p := &fakeProject{ID: fmt.Sprintf("p-%d", f.nextID),
			Name: body["name"].(string), Identifier: body["identifier"].(string)}
		f.projects = append(f.projects, p)
		writeJSON(w, p)
	case len(seg) >= 2 && seg[0] == "projects":
		f.serveProject(w, r, seg[1], seg[2:])
	default:
		http.NotFound(w, r)
	}
}

func (f *fakePlane) serveProject(w http.ResponseWriter, r *http.Request, pid string, seg []string) {
	switch {
	case len(seg) == 0 && r.Method == http.MethodPatch: // 项目 feature 开关等
		writeJSON(w, map[string]any{"id": pid, "cycle_view": true, "module_view": true})
	case len(seg) >= 1 && seg[0] == "issues":
		f.serveIssues(w, r, pid, seg[1:])
	case len(seg) == 1 && seg[0] == "states":
		writeJSON(w, map[string]any{"results": f.states, "next_cursor": ""})
	case len(seg) == 1 && seg[0] == "labels" && r.Method == http.MethodGet:
		writeJSON(w, map[string]any{"results": f.labels, "next_cursor": ""})
	case len(seg) == 1 && seg[0] == "labels" && r.Method == http.MethodPost:
		var body map[string]any
		decode(r, &body)
		f.nextID++
		l := &fakeLabel{ID: fmt.Sprintf("l-%d", f.nextID),
			Name: body["name"].(string), Color: body["color"].(string)}
		f.labels = append(f.labels, l)
		writeJSON(w, l)
	case len(seg) == 2 && seg[0] == "labels" && r.Method == http.MethodDelete:
		f.mu.Lock()
		out := f.labels[:0]
		for _, l := range f.labels {
			if l.ID != seg[1] {
				out = append(out, l)
			}
		}
		f.labels = out
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	case len(seg) >= 1 && seg[0] == "cycles":
		f.serveCycles(w, r, seg[1:])
	case len(seg) >= 1 && seg[0] == "modules":
		f.serveModules(w, r, seg[1:])
	default:
		http.NotFound(w, r)
	}
}

func (f *fakePlane) serveIssues(w http.ResponseWriter, r *http.Request, pid string, seg []string) {
	switch {
	case len(seg) == 0 && r.Method == http.MethodGet:
		writeJSON(w, map[string]any{"results": f.issues, "next_cursor": ""})
	case len(seg) == 0 && r.Method == http.MethodPost:
		var body map[string]any
		decode(r, &body)
		f.mu.Lock()
		f.nextIssueSeq++
		state, _ := body["state"].(string)
		if state == "" {
			state = "st-backlog"
		}
		priority, _ := body["priority"].(string)
		it := &fakeIssue{ID: fmt.Sprintf("iss-%d", f.nextIssueSeq), Project: pid,
			SequenceID: f.nextIssueSeq, Name: body["name"].(string),
			DescHTML: body["description_html"].(string), Priority: priority, State: state}
		if td, ok := body["target_date"].(string); ok {
			it.TargetDate = td
		}
		f.issues = append(f.issues, it)
		f.mu.Unlock()
		writeJSON(w, it)
	case len(seg) >= 1:
		it := f.issueByID(seg[0])
		if it == nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case len(seg) == 1 && r.Method == http.MethodGet:
			writeJSON(w, it)
		case len(seg) == 1 && r.Method == http.MethodPatch:
			var body map[string]any
			decode(r, &body)
			f.mu.Lock()
			f.patchIssue(it, body)
			f.mu.Unlock()
			writeJSON(w, it)
		case len(seg) == 1 && r.Method == http.MethodDelete:
			f.mu.Lock()
			f.removeIssue(it.ID)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case len(seg) >= 2 && seg[1] == "comments" && r.Method == http.MethodGet:
			writeJSON(w, map[string]any{"results": f.comments[it.ID], "next_cursor": ""})
		case len(seg) >= 2 && seg[1] == "comments" && r.Method == http.MethodPost:
			var body map[string]any
			decode(r, &body)
			f.mu.Lock()
			f.nextID++
			c := map[string]any{"id": fmt.Sprintf("c-%d", f.nextID), "actor": "u-me",
				"comment_html": body["comment_html"], "created_at": "2026-10-30T00:00:00Z"}
			f.comments[it.ID] = append(f.comments[it.ID], c)
			f.mu.Unlock()
			writeJSON(w, c)
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (f *fakePlane) patchIssue(it *fakeIssue, body map[string]any) {
	if v, ok := body["name"].(string); ok {
		it.Name = v
	}
	if v, ok := body["description_html"].(string); ok {
		it.DescHTML = v
	}
	if v, ok := body["priority"].(string); ok {
		it.Priority = v
	}
	if v, ok := body["state"].(string); ok {
		it.State = v
	}
	if v, ok := body["target_date"].(string); ok {
		it.TargetDate = v
	}
	if v, ok := body["assignees"].([]any); ok {
		it.Assignees = toStrSlice(v)
	}
	if v, ok := body["labels"].([]any); ok {
		it.Labels = toStrSlice(v)
	}
}

func toStrSlice(v []any) []string {
	out := make([]string, 0, len(v))
	for _, s := range v {
		if str, ok := s.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func (f *fakePlane) issueByID(id string) *fakeIssue {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, it := range f.issues {
		if it.ID == id {
			return it
		}
	}
	return nil
}

func (f *fakePlane) removeIssue(id string) {
	out := f.issues[:0]
	for _, it := range f.issues {
		if it.ID != id {
			out = append(out, it)
		}
	}
	f.issues = out
}

func (f *fakePlane) serveCycles(w http.ResponseWriter, r *http.Request, seg []string) {
	switch {
	case len(seg) == 0 && r.Method == http.MethodGet:
		writeJSON(w, map[string]any{"results": f.cycles, "next_cursor": ""})
	case len(seg) == 0 && r.Method == http.MethodPost:
		var body map[string]any
		decode(r, &body)
		f.mu.Lock()
		f.nextID++
		c := &fakeCycle{ID: fmt.Sprintf("cy-%d", f.nextID), Name: body["name"].(string),
			StartDate: body["start_date"].(string), EndDate: body["end_date"].(string)}
		f.cycles = append(f.cycles, c)
		f.mu.Unlock()
		writeJSON(w, c)
	case len(seg) == 1 && r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case len(seg) >= 2 && seg[1] == "cycle-issues" && len(seg) == 2 && r.Method == http.MethodGet:
		var items []*fakeIssue
		for _, id := range f.cycleItems[seg[0]] {
			if it := f.issueByID(id); it != nil {
				items = append(items, it)
			}
		}
		writeJSON(w, map[string]any{"results": items, "next_cursor": ""})
	case len(seg) >= 2 && seg[1] == "cycle-issues" && len(seg) == 2 && r.Method == http.MethodPost:
		var body map[string]any
		decode(r, &body)
		ids := toStrSlice(body["issues"].([]any))
		f.mu.Lock()
		f.cycleItems[seg[0]] = append(f.cycleItems[seg[0]], ids...)
		f.mu.Unlock()
		writeJSON(w, map[string]any{"issues": ids})
	case len(seg) == 3 && seg[1] == "cycle-issues" && r.Method == http.MethodDelete:
		f.mu.Lock()
		out := f.cycleItems[seg[0]][:0]
		for _, id := range f.cycleItems[seg[0]] {
			if id != seg[2] {
				out = append(out, id)
			}
		}
		f.cycleItems[seg[0]] = out
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakePlane) serveModules(w http.ResponseWriter, r *http.Request, seg []string) {
	switch {
	case len(seg) == 0 && r.Method == http.MethodGet:
		writeJSON(w, map[string]any{"results": f.modules, "next_cursor": ""})
	case len(seg) == 0 && r.Method == http.MethodPost:
		var body map[string]any
		decode(r, &body)
		f.mu.Lock()
		f.nextID++
		m := &fakeModule{ID: fmt.Sprintf("mo-%d", f.nextID), Name: body["name"].(string)}
		f.modules = append(f.modules, m)
		f.mu.Unlock()
		writeJSON(w, m)
	default:
		http.NotFound(w, r)
	}
}

func decode(r *http.Request, v *map[string]any) {
	_ = json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
