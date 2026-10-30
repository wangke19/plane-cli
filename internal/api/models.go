package api

// 与 Plane /api/v1/ 返回结构对应的最小模型集合，仅声明 CLI 用到的字段。

// Project 项目。
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Network     int    `json:"network"`
	CreatedAt   string `json:"created_at"`
}

// Issue 任务（work item）。注意：assignees/labels 在 v1 外部 API 里就是写入口径。
type Issue struct {
	ID              string   `json:"id"`
	Project         string   `json:"project"`
	SequenceID      int      `json:"sequence_id"`
	Name            string   `json:"name"`
	DescriptionHTML string   `json:"description_html"`
	Priority        string   `json:"priority"`
	State           string   `json:"state"`
	Assignees       []string `json:"assignees"`
	Labels          []string `json:"labels"`
	TargetDate      string   `json:"target_date"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// State 任务状态。
type State struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
	Sort  int    `json:"sort"`
}

// Label 标签。
type Label struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// Cycle 迭代周期。
type Cycle struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// Module 模块。
type Module struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	StartDate  string `json:"start_date"`
	TargetDate string `json:"target_date"`
}

// Member 工作区成员。
type Member struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// DisplayName 成员的展示名：优先姓名，缺省用 email。
func (m Member) DisplayName() string {
	name := m.FirstName + m.LastName
	if name == "" {
		return m.Email
	}
	return name
}

// Me 当前认证用户。
type Me struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
