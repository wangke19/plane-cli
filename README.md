<div align="center">

# plane-cli

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**[中文](#中文)** | **[English](#english)**

A GitHub-CLI-style command line tool for the [Plane](https://plane.so) project management platform.
一个 gh 风格的 Plane 项目管理命令行工具。

</div>

---

## 中文

`plane` 面向 AI Agent 与终端用户的日常操作：任务增删改查、指派、标签、周期、模块，一条命令搞定。

**零第三方依赖**：仅使用 Go 标准库（自带轻量参数解析器与 CJK 宽度对齐），无 cobra/color 等外部模块，离线可构建，长期可维护。

### 特性

- **任务全生命周期**：list / view / create / edit / close / reopen / delete / comment
- **协作字段**：assign（成员按 email/姓名/id 解析，`me` 指当前用户）、label（增删清）
- **迭代管理**：cycle create / add（任务进出周期）/ issues（周期内任务），module 管理
- **Agent 友好**：所有列表命令支持 `--json`；错误走 stderr、退出码 0/1/2；内置 429 限流退避重试
- **CJK 友好**：表格按东亚字符显示宽度对齐，中文列不串行
- **逃生舱**：`plane api` 可调用任意 REST 端点，覆盖 CLI 未封装的能力

### 安装

```bash
# 从源码构建（Go 1.27+）
git clone <repo-url> && cd plane-cli
make build          # 产物: bin/plane
make install        # 安装为 ~/.local/bin/plane-go
```

或直接用 go 构建：

```bash
go build -ldflags "-X main.version=$(git describe --tags --always)" -o plane ./cmd/plane
```

### 配置

凭据只从环境变量读取（代码与仓库不落盘任何真实地址与凭据）：

| 环境变量 | 必填 | 说明 |
|----------|------|------|
| `PLANE_TOKEN` | ✅ | Plane 个人 API Token（网页端 Settings → API Tokens 生成） |
| `PLANE_BASE_URL` | ✅ | 服务地址，如 `https://plane.example.com` |
| `PLANE_WORKSPACE` | ✅ | 工作区 slug |
| `PLANE_PROJECT` | — | 默认项目，设置后 `-p` 可省略 |

建议写入 `~/.bashrc`：

```bash
export PLANE_TOKEN="<your-token>"
export PLANE_BASE_URL="https://plane.example.com"
export PLANE_WORKSPACE="acme"
```

### 快速上手

```bash
plane auth status                                  # 检查认证
plane project list                                 # 项目列表
plane project create "新项目" -d "描述"            # 创建项目（自动开周期/模块视图）

PLANE_PROJECT=demo plane issue list                # 任务列表（--state todo --priority high 可筛选）
plane issue view 8 -p demo                         # 详情（描述已转纯文本）
plane issue create -p demo -t "标题" -d "描述" --priority high --due 2026-12-01
plane issue edit 8 -p demo -s doing                # 改状态（别名: todo/backlog/doing/done）
plane issue assign 8 me --remove bot               # 指派负责人
plane issue label 8 bug perf --remove docs         # 打标签
plane issue close 8 -p demo                        # → Done；reopen 反向

plane cycle create -p demo -n "Sprint1" --start 2026-12-14 --end 2026-12-27
plane cycle add Sprint1 8 -p demo                  # 任务进周期（--remove 移出）
plane cycle issues Sprint1 -p demo                 # 周期内任务

plane module list -p demo                          # 模块
plane label create bug --color "#ff0000" -p demo   # 建标签

plane api GET workspaces/acme/projects/ --raw      # 任意 REST 端点（逃生舱）
```

### 设计说明

- **参数解析**：自研 `flagSet`（`internal/app/flags.go`），支持 flag 与位置参数任意交错、
  `--flag=value`、短/长别名、`--` 终止符——覆盖 gh 风格用法，同时保持零依赖。
- **分页**：`Paginate[T]` 泛型函数跟随 `next_cursor` 游标翻页，兼容信封与裸数组两种端点。
- **限流**：429 时按 `Retry-After` 与指数退避（2s/4s/8s）自动重试至多 3 次。
- **版本注入**：`-ldflags "-X main.version=$(git describe ...)"`，`plane --version` 可查。

### 已知边界（针对 Plane v1.4.x 自托管）

- 外部 `/api/v1/`（PAT 认证）写状态/指派/标签的字段是 `state`/`assignees`/`labels`；
  内部 `/api/`（session 认证）才是 `state_id`/`assignee_ids`——传错会被**静默忽略**（返回 200）。
- 通过 API 创建的项目默认关闭周期/模块视图，需 PATCH 项目 `{"cycle_view": true, "module_view": true}`
  （`project create` 已自动处理）。
- 项目名禁止连字符等特殊字符；标识符仅大写字母数字。
- 任务归档（archive）在外部 v1 API 无端点，仅网页可用。

### 开发

```bash
make fmt && make vet && make test   # 提交前三连
```

项目结构（标准 Go 布局）：

```
cmd/plane/          入口
internal/config/    环境变量配置
internal/textutil/  CJK 宽度、表格对齐、HTML↔文本
internal/api/       HTTP 客户端（重试/分页）、模型、资源解析
internal/app/       参数解析、命令分发、各子命令
```

---

## English

`plane` is built for the daily workflow of AI agents and terminal users: full work-item CRUD,
assignees, labels, cycles, and modules — one command each.

**Zero third-party dependencies**: standard library only (ships its own lightweight flag parser and
CJK-aware table alignment). No cobra, no color — builds offline and stays maintainable.

### Features

- **Full work-item lifecycle**: list / view / create / edit / close / reopen / delete / comment
- **Collaboration fields**: assign (members resolved by email/name/id, `me` = current user) and label (add/remove/clear)
- **Iteration management**: cycle create / add (move items in and out) / issues, plus module management
- **Agent friendly**: every list command supports `--json`; errors go to stderr with exit codes 0/1/2; automatic 429 rate-limit backoff
- **CJK friendly**: tables align by East Asian display width — Chinese columns never misalign
- **Escape hatch**: `plane api` calls any REST endpoint, covering everything the CLI does not wrap

### Installation

```bash
# Build from source (Go 1.27+)
git clone <repo-url> && cd plane-cli
make build          # output: bin/plane
make install        # installs to ~/.local/bin/plane-go
```

Or build with go directly:

```bash
go build -ldflags "-X main.version=$(git describe --tags --always)" -o plane ./cmd/plane
```

### Configuration

Credentials are read from environment variables only (no addresses or secrets are stored in this
repository):

| Variable | Required | Description |
|----------|----------|-------------|
| `PLANE_TOKEN` | ✅ | Personal API token (Plane web → Settings → API Tokens) |
| `PLANE_BASE_URL` | ✅ | Instance URL, e.g. `https://plane.example.com` |
| `PLANE_WORKSPACE` | ✅ | Workspace slug |
| `PLANE_PROJECT` | — | Default project; makes `-p` optional |

Add this to `~/.bashrc`:

```bash
export PLANE_TOKEN="<your-token>"
export PLANE_BASE_URL="https://plane.example.com"
export PLANE_WORKSPACE="acme"
```

### Quick start

```bash
plane auth status                                  # verify credentials
plane project list                                 # list projects
plane project create "New Project" -d "desc"       # create (enables cycles/modules)

PLANE_PROJECT=demo plane issue list                # filters: --state todo --priority high
plane issue view 8 -p demo                         # detail (description rendered as text)
plane issue create -p demo -t "Title" -d "Body" --priority high --due 2026-12-01
plane issue edit 8 -p demo -s doing                # state aliases: todo/backlog/doing/done
plane issue assign 8 me --remove bot               # manage assignees
plane issue label 8 bug perf --remove docs         # manage labels
plane issue close 8 -p demo                        # → Done; reopen does the reverse

plane cycle create -p demo -n "Sprint1" --start 2026-12-14 --end 2026-12-27
plane cycle add Sprint1 8 -p demo                  # add item to cycle (--remove to take out)
plane cycle issues Sprint1 -p demo                 # items inside a cycle

plane module list -p demo                          # modules
plane label create bug --color "#ff0000" -p demo   # create a label

plane api GET workspaces/acme/projects/ --raw      # any REST endpoint (escape hatch)
```

### Design notes

- **Argument parsing**: a hand-rolled `flagSet` (`internal/app/flags.go`) supporting flags interleaved
  with positionals, `--flag=value`, short/long aliases, and the `--` terminator — gh-style UX with
  zero dependencies.
- **Pagination**: the generic `Paginate[T]` follows `next_cursor` and supports both enveloped and
  bare-array endpoints.
- **Rate limiting**: on 429 the client retries up to 3 times, honoring `Retry-After` with
  exponential backoff (2s/4s/8s).
- **Version injection**: `-ldflags "-X main.version=$(git describe ...)"`, exposed via `plane --version`.

### Known limitations (self-hosted Plane v1.4.x)

- On the external `/api/v1/` (PAT auth), state/assignees/labels are written via the `state` /
  `assignees` / `labels` fields; the internal `/api/` (session auth) expects `state_id` /
  `assignee_ids` — wrong fields are **silently ignored** (HTTP 200, no change).
- Projects created through the API start with cycles/modules disabled; PATCH the project with
  `{"cycle_view": true, "module_view": true}` (`project create` handles this automatically).
- Project names forbid hyphens and other special characters; identifiers are uppercase alphanumeric only.
- Work-item archive has no external v1 endpoint; it is web-UI only.

### Development

```bash
make fmt && make vet && make test   # run all three before committing
```

Project layout (standard Go):

```
cmd/plane/          entrypoint
internal/config/    environment configuration
internal/textutil/  CJK width, table alignment, HTML↔text
internal/api/       HTTP client (retry/pagination), models, resolvers
internal/app/       flag parsing, dispatch, subcommands
```

## License

[MIT](LICENSE)
