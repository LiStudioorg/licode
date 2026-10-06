# licode —— AI 编程助手（Web 版）

用 Go 编写的 AI 编程助手，单二进制、静态编译、跨平台。浏览器打开即用，手机/电脑均可使用。

> **重要声明：发行版可能并不代表最新版本，如想使用最新代码，请自行构建。**

## 功能

- 多 AI 提供商一键切换：**OpenAI / Claude / Ollama / Gemini**（均用各自原生接口），可添加多个厂商并一键获取模型列表
- 工具调用：读写文件、目录、代码搜索、shell 执行，Agent 自主调用并回填；工具规则可配置（允许/询问/拒绝），可"始终允许"，可自动允许
- 子代理系统：explorer / builder / planner，DAG 依赖并行调度
- 多对话：会话列表、自动标题、切换、删除，实时保存到 `~/.licode/sessions/`
- MCP 接入（本地命令 stdio 与远程 http 均可）与 Skills 技能（可多个，`skills/*.md`）
- 上下文压缩 compaction、自动标题 title_gen
- 运行时健壮性：LLM 调用指数退避重试（处理 429/503/网络抖动）、子代理硬超时、上下文窗口滑动保护（token 预算）
- 安全纵深：工具输出敏感信息脱敏（sk-* API Key 等）、可选 Docker 沙箱隔离执行 Shell
- 可观测性：结构化 JSON 日志（trace_id 串联 Agent→Tool 调用链，`LICODE_JSON_LOG=1`）
- 备份迁移：一键导出/导入 zip（配置 + 会话 + 技能 + 附加提示词）
- 热重载：SIGHUP 重载配置（修改 `~/.licode/config.json` 后 `kill -HUP <pid>` 即时生效，不中断服务）
- 浅色/深色主题切换，流式工具调用渲染（参数/结果折叠卡片），生成中可随时"停止"
- 系统提示词读取 `~/.licode/system-prompt.md`；`md/` 目录递归读取所有 `.md` 作为附加提示词
- 离线前端：Vite + Vue 3 + Pinia 静态 SPA（由 `web/` 生成），JS/CSS 全部随二进制打包（`go:embed`），无任何 CDN 外部依赖，内网/离线环境开箱即用
- 响应式界面：桌面三栏（会话列表 / 对话 / 文件树），手机与平板自动切换为抽屉式单栏，软键盘弹出不遮挡输入框
- 登录认证（默认用户名 `licode`，密码自行设置；未启用登录时页面会提醒如何启用）
- HTTPS（`--https`，无证书时自动生成自签名证书）
- 数据按系统用户隔离：每个系统用户在自己的家目录 `~/.licode` 运行，互不影响

## 安装

### 方式一：从 Release 下载（推荐）

到 [Releases](https://github.com/LiStudioorg/licode/releases) 页面下载对应平台的二进制，赋权后直接运行，无需安装依赖：

```bash
# 以 Linux amd64 为例（把 URL 里的版本换成你要的版本）
curl -L -o licode \
  https://github.com/LiStudioorg/licode/releases/download/v0.5.0/licode-linux-amd64
chmod +x licode
./licode
```

一行命令自动识别平台（Linux / macOS）：

```bash
VERSION=v0.5.0
OS=$(uname -s | tr '[:upper:]' '[:lower:]')          # linux / darwin
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -L -o licode \
  "https://github.com/LiStudioorg/licode/releases/download/${VERSION}/licode-${OS}-${ARCH}"
chmod +x licode && ./licode
```

可用平台：`linux`（amd64 / arm64 / 386）、`darwin`（amd64 / arm64）、`windows`（amd64 / arm64 / 386，后缀 `.exe`）、`freebsd`（amd64）。

每个 Release 附 `checksums.txt`，可校验下载是否完整（注意下载时要用**原始文件名**，校验命令按清单里的名字匹配）：

```bash
curl -L -O https://github.com/LiStudioorg/licode/releases/download/v0.5.0/checksums.txt
curl -L -O https://github.com/LiStudioorg/licode/releases/download/v0.5.0/licode-linux-amd64
sha256sum -c checksums.txt --ignore-missing   # macOS 用 shasum -a 256 -c
# → licode-linux-amd64: OK

# 若已重命名为 licode，则手工比对：
# sha256sum licode   然后与 checksums.txt 中对应行对照
```

安装到 PATH（可选）：

```bash
mkdir -p ~/.local/bin
install -m 755 licode-linux-amd64 ~/.local/bin/licode
# 若 ~/.local/bin 不在 PATH 中，追加一行到 shell 配置：
# echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
```

### 方式二：从源码构建

```bash
git clone https://github.com/LiStudioorg/licode.git
cd licode
./build.sh                  # 9 平台交叉编译，产物在 build/
./build/licode-linux-amd64  # 启动
```

`build.sh` 默认**不重建前端**，直接使用仓库中已提交的 `internal/web/dist`（因此无需 Node.js，纯 Go 即可构建）。若改动了 `web/src/**` 需要重新编译前端：

```bash
cd web && npm install && npm run build   # 产出 web/dist
LICODE_FRONTEND_BUILD=1 bash scripts/sync-frontend.sh   # 同步到 internal/web/dist
cd .. && ./build.sh
```

## 快速开始

```bash
./licode                    # 默认监听 127.0.0.1:8080
 
# 参数
./licode --host 0.0.0.0 --port 8080    # 局域网/手机访问
./licode --password mypass             # 启用登录（默认用户名 licode）
```

浏览器打开 `http://127.0.0.1:8080` 使用；手机访问请用 `--host 0.0.0.0` 并打开 `http://服务器IP:8080`。

> ⚠️ 未设置 `--password` 时**不启用登录**。绑到 `0.0.0.0` 且无密码，意味着同网段任何人都能操作这个**可执行 Shell 命令**的 Agent，请务必配合 `--password` 使用。

## 工具与权限
 
| 工具 | 说明 | 默认权限 |
| --- | --- | --- |
| `Read` | 读取文件（支持 offset/limit） | 允许 |
| `Write` | 写入/替换文件 | **需审批** |
| `Edit` | 查找替换编辑文件（精确修改） | 允许 |
| `ListDirectory` | 列出目录内容 | 允许 |
| `Grep` | 正则搜索代码 | 允许 |
| `Glob` | 按通配符查找文件 | 允许 |
| `Shell` | 执行 shell 命令（构建/测试/git） | **需审批** |
| `Delete` | 删除文件或空目录 | **需审批** |
| `Move` | 移动/重命名文件 | 允许 |
| `Dispatch` | 并行调度子代理执行任务 | 允许 |

> **Skills / MCP 不是"需要手动加载"的东西**：把它们放进对应目录就自动加载、运行中热更新。

**权限配置**（设置 → 工具规则，如 `Read:allow, Write:ask, Bash:deny`）：

- `allow` 允许 · `ask` 避审批 · `deny` 禁用
- 需审批时前端弹出「拒绝 / 允许 / **始终允许**」——始终允许**仅当前对话生效**

## 设置

无需重启，网页端「设置」里实时修改并自动写回 `~/.licode/config.json`：

### 启动参数说明（全部为可选项）

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `--host` | 监听主机。默认 `127.0.0.1`（仅本机）。手机/局域网访问请用 `0.0.0.0` | `--host 0.0.0.0` |
| `--port` | 监听端口，默认 `8080` | `--port 8080` |
| `--username` | 登录用户名，默认 `licode` | `--username admin` |
| `--password` | 登录密码；**设置后才启用登录** | `--password mypass` |
| `--https` | 启用 HTTPS（无证书时自动生成自签名证书） | `--https` |
| `--tls-cert` / `--tls-key` | 指定 TLS 证书/私钥文件 | `--tls-cert cert.pem --tls-key key.pem` |
| `--no-subagents` | 禁用子代理编排 | `--no-subagents` |
| `-c` / `--config` | 配置文件路径（默认 `config.toml`） | `-c /path/to/config.toml` |

### 环境变量说明

| 环境变量 | 说明 |
| --- | --- |
| `LICODE_USERNAME` | 登录用户名（同 `--username`） |
| `LICODE_PASSWORD` | 登录密码（同 `--password`） |
| `LICODE_HOME` | 用户数据目录（默认 `~/.licode`） |
| `LICODE_JSON_LOG` | 置为 `1` 输出结构化 JSON 日志（含 trace_id） |

### 提供商默认值

| 提供商 | 默认地址 | 默认模型 |
| --- | --- | --- |
| openai | `https://api.openai.com/v1` | `gpt-4o-mini` |
| claude | `https://api.anthropic.com` | `claude-sonnet-4-20250514` |
| ollama | `http://localhost:11434` | `llama3.1:8b` |
| gemini | `https://generativelanguage.googleapis.com` | `gemini-2.0-flash` |

## 登录

`--password` 或 `LICODE_PASSWORD` 设置密码后启用登录界面（默认用户名 `licode`）。登录基于会话 Cookie（HMAC 签名），网页与 WebSocket 均受保护。

## 数据目录

`~/.licode` 首次使用自动生成：

```
~/.licode/
├── config.json      配置文件（AI 设置等，网页端「设置」写回这里）
├── config.toml      服务器选项（host/port/登录等启动期配置，由 -c 或默认路径读取）
├── session.key      会话 Cookie 签名密钥（自动生成，请勿泄露）
├── skills/          技能（markdown）
├── mcp/             MCP 服务器配置
├── plugins/         插件
├── tools/           外部工具
├── sessions/        对话记录（实时保存）
├── logs/            日志
├── cache/           缓存
├── md/              附加提示词（递归读取其中所有 .md）
└── system-prompt.md 系统提示词（首次自动生成默认内容）
```

## 文档

- [⚙️ 配置与数据目录](docs/config.md)
- [🧠 多厂商与模型](docs/providers.md)
- [🤖 子代理系统](docs/agents.md)
- [🌐 Web 界面与 API](docs/web.md)
- [🛠 开发与构建](docs/develop.md)
- [❓ 常见问题](docs/faq.md)

## 构建

```bash
./build.sh      # 9 平台交叉编译，产物在 build/（CGO_ENABLED=0 静态编译）
```

单二进制约 12 MB（含内嵌前端）；完全静态编译，不依赖 glibc；空闲内存 < 30 MB。

推 `v*` 标签会触发 GitHub Actions 自动跑质量门禁（`go vet` + `go test`）并发布上述 9 个平台产物到 Release，详见 `.github/workflows/release.yml`。

## 架构

```
├── main.go                # 入口
├── cmd/
│   ├── serve.go           # Web 服务器 + WebSocket + 路由
│   ├── auth.go            # 登录认证
│   └── files.go           # 文件浏览/编辑 API
├── internal/
│   ├── ai/                # LLMClient 接口 + openai/claude/ollama/gemini
│   ├── agent/             # 主 Agent、工具、子代理 DAG、MCP、Skills
│   ├── session/           # 多会话 + 实时落盘
│   ├── settings/          # 设置 + ~/.licode 数据目录
│   ├── websocket/         # Hub + 事件协议
│   └── web/               # go:embed 内嵌前端产物（由 web/ 构建后同步）
├── web/                   # 前端工程（Vite + Vue 3 + Pinia + Tailwind v4）
│   ├── src/components/    # 布局外壳、输入区、状态栏、文件树等
│   ├── src/views/         # 路由页：对话 / 文件 / 设置 / 诊断 / 登录
│   └── src/stores/        # Pinia：会话、设置、界面状态
└── build.sh               # 9 平台交叉编译
```

## 开发

```bash
go build ./...     # 编译检查
go vet ./...       # 静态检查
go test ./...      # 单元测试
```

## 联系与交流

- **GitHub**：https://github.com/LiStudioorg/licode
- **Gitee**：https://gitee.com/li63050a/licode
- **开发者 B 站**：[小帅5656](https://b23.tv/nDqj0DT) — 关注获取最新动态、教程、演示
- **QQ 技术交流群**：[点击加入](https://qun.qq.com/universal-share/share?ac=1&authKey=zq9BYcTtBQm6GbvWiEWiBvDWNWbqhw2%2F%2BRnGM21c0jcL%2FofGqBFeXLr%2BtYT3SkO6&busi_data=eyJncm91cENvZGUiOiIxMDI2OTM5NzQxIiwidG9rZW4iOiJxNkNWUTUxYXVxSmRHZXRvdWtkZnhaN25INzJrMmNaNFpVTjJ5ZTVLYmRvWTFuOEZTd093UXBtQi8vQWk2T1JyIiwidWluIjoiMzYzNTczNjE4MCJ9&data=073ZrPEFZXFvoEDWatbWTidAitiN4OIbiaVDWoR7hVIwJurEPC7Swm6OREVpn6omzobXLn3SRErNKxKbYDTZQA&svctype=4&tempid=h5_group_info)（群号：1026939741）— 提问、反馈 bug、讨论功能
- **开发者邮箱**：li63050@qq.com
