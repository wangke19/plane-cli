package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// 本包提供各类资源的"引用 → 完整对象"解析：
// 支持名称/标识符/id 前缀等宽松写法，与项目内任务的 sequence 编号。

// Me 返回当前认证用户。
func (c *Client) Me() (*Me, error) {
	resp, err := c.Do(http.MethodGet, "users/me/", nil)
	if err != nil {
		return nil, err
	}
	me := &Me{}
	if err := resp.Decode(me); err != nil {
		return nil, err
	}
	return me, nil
}

// Projects 返回工作区全部项目。
func (c *Client) Projects() ([]Project, error) {
	return Paginate[Project](c, fmt.Sprintf("workspaces/%s/projects/", c.Workspace), 500)
}

// ResolveProject 按 名称/标识符/slug/id 前缀（大小写不敏感）解析项目。
func (c *Client) ResolveProject(key string) (*Project, error) {
	k := strings.ToLower(strings.TrimSpace(key))
	projects, err := c.Projects()
	if err != nil {
		return nil, err
	}
	for i := range projects {
		p := &projects[i]
		if strings.HasPrefix(p.ID, k) ||
			k == strings.ToLower(p.Name) ||
			k == strings.ToLower(p.Identifier) ||
			k == strings.ToLower(p.Slug) {
			return p, nil
		}
	}
	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name
	}
	return nil, fmt.Errorf("找不到项目 '%s'。现有项目: %s", key, strings.Join(names, ", "))
}

// Issues 返回项目全部任务。
func (c *Client) Issues(projectID string) ([]Issue, error) {
	return Paginate[Issue](c, fmt.Sprintf("workspaces/%s/projects/%s/issues/", c.Workspace, projectID), 1000)
}

// ResolveIssue 按项目内 sequence 编号或 id 前缀解析任务。
func (c *Client) ResolveIssue(projectID, ref string) (*Issue, error) {
	ref = strings.TrimSpace(ref)
	issues, err := c.Issues(projectID)
	if err != nil {
		return nil, err
	}
	isNum := regexp.MustCompile(`^\d+$`).MatchString(ref)
	for i := range issues {
		it := &issues[i]
		if isNum && it.SequenceID == atoiSafe(ref) {
			return it, nil
		}
		if !isNum && len(ref) >= 8 && strings.HasPrefix(strings.ToLower(it.ID), strings.ToLower(ref)) {
			return it, nil
		}
	}
	return nil, fmt.Errorf("找不到任务 '%s'（项目内按编号或 id 前缀匹配）", ref)
}

func atoiSafe(s string) int {
	n := 0
	for _, ch := range s {
		n = n*10 + int(ch-'0')
	}
	return n
}

// States 返回项目状态列表。
func (c *Client) States(projectID string) ([]State, error) {
	resp, err := c.Do(http.MethodGet, fmt.Sprintf("workspaces/%s/projects/%s/states/", c.Workspace, projectID), nil)
	if err != nil {
		return nil, err
	}
	var env page[State]
	if err := resp.Decode(&env); err != nil {
		return nil, err
	}
	if env.Results == nil {
		var arr []State
		if err := resp.Decode(&arr); err != nil {
			return nil, err
		}
		env.Results = arr
	}
	return env.Results, nil
}

var stateAliases = map[string]string{
	"todo": "unstarted", "backlog": "backlog",
	"in-progress": "started", "doing": "started", "progress": "started",
	"done": "completed", "closed": "completed",
	"cancelled": "cancelled", "canceled": "cancelled",
}

// ResolveState 按名称、id 或 group 别名（todo/doing/done…）解析状态。
func (c *Client) ResolveState(projectID, s string) (*State, error) {
	states, err := c.States(projectID)
	if err != nil {
		return nil, err
	}
	k := strings.ToLower(strings.TrimSpace(s))
	if grp, ok := stateAliases[k]; ok {
		for i := range states {
			if states[i].Group == grp {
				return &states[i], nil
			}
		}
	}
	for i := range states {
		if k == strings.ToLower(states[i].Name) || strings.HasPrefix(states[i].ID, k) {
			return &states[i], nil
		}
	}
	names := make([]string, len(states))
	for i, st := range states {
		names[i] = fmt.Sprintf("%s(%s)", st.Name, st.Group)
	}
	return nil, fmt.Errorf("找不到状态 '%s'。可用: %s", s, strings.Join(names, ", "))
}

// StateByGroup 返回项目内指定 group 的第一个状态（close→completed、reopen→unstarted 用）。
func (c *Client) StateByGroup(projectID, group string) (*State, error) {
	states, err := c.States(projectID)
	if err != nil {
		return nil, err
	}
	for i := range states {
		if states[i].Group == group {
			return &states[i], nil
		}
	}
	return nil, fmt.Errorf("项目没有 group=%s 的状态", group)
}

// Members 返回工作区成员列表。
func (c *Client) Members() ([]Member, error) {
	return Paginate[Member](c, fmt.Sprintf("workspaces/%s/members/", c.Workspace), 500)
}

// ResolveMember 按 email/姓名/id 前缀解析成员 id；'me' 指当前用户。
func (c *Client) ResolveMember(key string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "me" {
		me, err := c.Me()
		if err != nil {
			return "", err
		}
		return me.ID, nil
	}
	members, err := c.Members()
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if strings.EqualFold(m.Email, k) || strings.HasPrefix(m.ID, k) {
			return m.ID, nil
		}
	}
	for _, m := range members {
		if strings.Contains(strings.ToLower(m.DisplayName()), k) {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("找不到成员 '%s'（按 email/姓名/id 前缀匹配）", key)
}

// MemberName 把成员 id 列表转为展示名，解析失败时回退 id 前 8 位。
func (c *Client) MemberName(id string) string {
	members, err := c.Members()
	if err == nil {
		for _, m := range members {
			if m.ID == id {
				return m.DisplayName()
			}
		}
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Labels 返回项目全部标签。
func (c *Client) Labels(projectID string) ([]Label, error) {
	return Paginate[Label](c, fmt.Sprintf("workspaces/%s/projects/%s/labels/", c.Workspace, projectID), 500)
}

// ResolveLabel 按名称或 id 前缀解析标签。
func (c *Client) ResolveLabel(projectID, name string) (*Label, error) {
	labels, err := c.Labels(projectID)
	if err != nil {
		return nil, err
	}
	k := strings.ToLower(strings.TrimSpace(name))
	for i := range labels {
		if strings.EqualFold(labels[i].Name, k) || strings.HasPrefix(labels[i].ID, k) {
			return &labels[i], nil
		}
	}
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	existing := strings.Join(names, ", ")
	if existing == "" {
		existing = "（无）"
	}
	return nil, fmt.Errorf("找不到标签 '%s'。现有: %s（可先 label create 创建）", name, existing)
}

// Cycles 返回项目全部周期。
func (c *Client) Cycles(projectID string) ([]Cycle, error) {
	return Paginate[Cycle](c, fmt.Sprintf("workspaces/%s/projects/%s/cycles/", c.Workspace, projectID), 200)
}

// ResolveCycle 按名称或 id 前缀解析周期。
func (c *Client) ResolveCycle(projectID, key string) (*Cycle, error) {
	cycles, err := c.Cycles(projectID)
	if err != nil {
		return nil, err
	}
	k := strings.ToLower(strings.TrimSpace(key))
	for i := range cycles {
		if strings.HasPrefix(cycles[i].ID, k) || strings.Contains(strings.ToLower(cycles[i].Name), k) {
			return &cycles[i], nil
		}
	}
	names := make([]string, len(cycles))
	for i, cy := range cycles {
		names[i] = cy.Name
	}
	existing := strings.Join(names, ", ")
	if existing == "" {
		existing = "（无）"
	}
	return nil, fmt.Errorf("找不到周期 '%s'。现有: %s", key, existing)
}

// CycleIssues 列出周期内的任务。
func (c *Client) CycleIssues(projectID, cycleID string) ([]Issue, error) {
	return Paginate[Issue](c, fmt.Sprintf("workspaces/%s/projects/%s/cycles/%s/cycle-issues/", c.Workspace, projectID, cycleID), 200)
}

// Modules 返回项目全部模块。
func (c *Client) Modules(projectID string) ([]Module, error) {
	return Paginate[Module](c, fmt.Sprintf("workspaces/%s/projects/%s/modules/", c.Workspace, projectID), 200)
}

// ResolveModule 按名称或 id 前缀解析模块。
func (c *Client) ResolveModule(projectID, key string) (*Module, error) {
	modules, err := c.Modules(projectID)
	if err != nil {
		return nil, err
	}
	k := strings.ToLower(strings.TrimSpace(key))
	for i := range modules {
		if strings.HasPrefix(modules[i].ID, k) || strings.Contains(strings.ToLower(modules[i].Name), k) {
			return &modules[i], nil
		}
	}
	names := make([]string, len(modules))
	for i, m := range modules {
		names[i] = m.Name
	}
	existing := strings.Join(names, ", ")
	if existing == "" {
		existing = "（无）"
	}
	return nil, fmt.Errorf("找不到模块 '%s'。现有: %s", key, existing)
}
