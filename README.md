licode

AI 编程助手，只有 Web 界面。 用 Go 编写，单二进制静态编译，浏览器打开即用，手机和电脑均可访问。支持 OpenAI、Claude、Gemini、Ollama 等多家厂商，内置工具调用、子代理、MCP 接入与插件系统。

重要声明：发行版可能并不代表最新版本。如需使用最新代码，请自行构建。

---

快速开始

```bash
git clone https://github.com/LiStudioorg/licode.git
cd licode
./build.sh
```

编译完成后启动：

```bash
# 本机使用
./build/licode-linux-amd64

# 局域网 / 手机访问
./build/licode-linux-amd64 --host 0.0.0.0 --port 8080

# 启用登录（默认用户名 licode）
./build/licode-linux-amd64 --password mypass
```

浏览器打开 http://127.0.0.1:8080 即可使用。

构建要求

· Go ≥ 1.19
· 纯 Go 依赖，无 CGO：fsnotify、gorilla/websocket、cobra、BurntSushi/toml

---

功能

· 多 AI 提供商：OpenAI / Claude / Gemini / Ollama，均使用各自原生接口，可添加多个厂商并一键获取模型列表
· 工具调用：读写文件、目录浏览、代码搜索、Shell 执行，Agent 自主调用并回填结果；工具规则可配置（允许 / 询问 / 拒绝），支持“始终允许”与自动允许
· 子代理系统：内置 explorer / builder / planner 三种子代理，支持 DAG 依赖并行调度
· 多对话：会话列表、自动标题、切换、删除，实时保存至 ~/.licode/sessions/
· MCP 接入：本地命令（stdio）与远程 HTTP 均可接入
· Skills 技能：以 Markdown 形式放置于 skills/ 目录自动加载
· 上下文压缩：compaction 与自动标题 title_gen 可配置开关
· 运行时健壮性：LLM 调用指数退避重试（处理 429/503/网络抖动）、子代理硬超时、上下文窗口滑动保护
· 安全：工具输出敏感信息脱敏（如 sk-* API Key）；登录认证（默认用户名 licode，密码自行设置）；HTTPS 支持（--https，无证书时自动生成自签名证书）
· 可观测性：结构化 JSON 日志，trace_id 串联 Agent → Tool 调用链（LICODE_JSON_LOG=1）
· 备份迁移：一键导出 / 导入 zip（配置 + 会话 + 技能 + 附加提示词）
· 热重载：SIGHUP 重载配置（修改 ~/.licode/config.json 后 kill -HUP <pid> 即时生效，不中断服务）
· 前端：Nuxt 3 + Vue 静态 SPA（由 nuxtweb/ 生成），JS/CSS 全部通过 go:embed 打包进二进制，无 CDN 外部依赖，内网 / 离线环境开箱即用
· 数据隔离：按系统用户隔离，每个用户在自己的家目录 ~/.licode 运行

---

工具与权限

工具 说明 默认权限
Read 读取文件（支持 offset/limit） 允许
Write 写入 / 替换文件 需审批
Edit 查找替换编辑文件（精确修改） 允许
ListDirectory 列出目录内容 允许
Grep 正则搜索代码 允许
Glob 按通配符查找文件 允许
Shell 执行 shell 命令（构建 / 测试 / git） 需审批
Delete 删除文件或空目录 需审批
Move 移动 / 重命名文件 允许
Dispatch 并行调度子代理执行任务 允许

权限配置位于「设置 → 工具规则」，格式如 Read:allow, Write:ask, Shell:deny。需审批时前端弹出「拒绝 / 允许 / 始终允许」——“始终允许”仅当前对话生效。

---

启动参数

参数 说明 示例
--host 监听主机（默认 127.0.0.1） --host 0.0.0.0
--port 端口（默认 8080） --port 8080
--addr 直接指定监听地址（优先于 host/port） --addr 0.0.0.0:9000
--provider 厂商：openai / claude / google / ollama --provider ollama
--base-url API 地址 --base-url http://localhost:11434/v1
--api-key API 密钥 --api-key sk-xxx
--model 模型名 --model gpt-4o-mini
--username 登录用户名（默认 licode） --username admin
--password 登录密码（设置后才启用登录） --password mypass
--https 启用 HTTPS（无证书时自动生成自签名证书） --https
--tls-cert / --tls-key 指定 TLS 证书 / 私钥文件 --tls-cert cert.pem --tls-key key.pem
--no-subagents 禁用子代理编排 --no-subagents
-h / --help 显示简体中文帮助 ./licode --help

优先级：命令行 > 环境变量 > 配置文件 > 默认值。

---

环境变量

变量 说明
LICODE_PROVIDER 厂商（同 --provider）
LICODE_BASE_URL API 地址
LICODE_API_KEY API 密钥
LICODE_MODEL 模型名
LICODE_USERNAME 登录用户名
LICODE_PASSWORD 登录密码
LICODE_HOME 数据目录（默认 ~/.licode）
LICODE_JSON_LOG 置为 1 输出结构化 JSON 日志
OPENAI_API_KEY / ANTHROPIC_API_KEY / GEMINI_API_KEY 各厂商标准密钥（未设 LICODE_API_KEY 时自动读取）



---

数据目录

~/.licode 首次使用自动生成：

```
~/.licode/
├── config.json          设置（providers、tool_rules、mcp_servers 等）
├── system-prompt.md     系统提示词（可直接编辑，首次自动生成默认内容）
├── md/                  附加提示词（递归读取其中所有 .md 追加到系统提示词）
├── skills/              技能（Markdown，frontmatter: name/description）
├── mcp/                 MCP 服务器配置
├── sessions/            对话记录（实时保存，每会话一个 json）
├── logs/                日志
├── cache/               缓存
└── ssl/                 HTTPS 自签名证书
```

项目内可放置 licode.json 覆盖用户级配置。数据按系统用户隔离（各用户家目录独立）。

config.json 示例：

```json
{
  "provider": "claude",
  "model": "claude-sonnet-4-20250514",
  "base_url": "https://api.anthropic.com",
  "api_key": "sk-ant-xxx",
  "temperature": 0.7,
  "max_tokens": 4096,
  "max_iterations": 16,
  "subagents": true,
  "compaction": true,
  "title_gen": true,
  "auto_allow": false,
  "tool_rules": { "write_file": "ask", "run_shell": "ask" },
  "providers": [
    {
      "provider": "openai",
      "name": "OpenAI",
      "type": "openai",
      "base_url": "https://api.openai.com/v1",
      "api_key": "",
      "model": "gpt-4o-mini"
    }
  ]
}
```



---

子代理系统

主 Agent 面对复杂任务时，可拆解为多个独立子任务交给专门子代理并行执行。

名称 职责 工具
explorer 探索代码库，给出带 文件:行号 的结论 read_file, list_dir, glob, grep
builder 实施改动并用构建 / 测试验证 read_file, write_file, list_dir, grep, glob, run_shell
planner 制定分步实施计划（不写代码） 无



通过 dispatch_subagents 工具一次性提交多个任务，支持 depends_on 形成 DAG 依赖。相互独立的任务在同一层并行执行（goroutine），依赖环会报告错误，全部完成后由主 Agent 汇总。

自定义子代理：在 ~/.licode/agents/*.md 编写，frontmatter 声明名称、描述、工具，正文为 System Prompt。

---

插件系统（Cordis 范式）

licode 的插件系统基于 dsh / Cordis 范式，核心设计原则：

· 可逆副作用：插件加载时注册的任何资源，卸载时由框架自动按 LIFO 顺序回滚，插件作者无需手写清理逻辑
· 无特权核心：AI Provider、工具注册表、会话管理、甚至主循环本身都是普通插件
· 响应式依赖：插件通过 inject 声明所需服务，服务消失时自动卸载，恢复后自动重新加载



内置插件（编译进二进制）包括 builtin-llm-openai、builtin-llm-claude、builtin-tools-fs、builtin-tools-shell、builtin-agent-loop、builtin-subagent、builtin-mcp。

第三方插件（外部进程，任意语言）安装到 ~/.licode/plugins/ 即可自动加载。详见 PLUGIN_SYSTEM.md 与 PLUGIN_DEV.md。

---

项目结构

```
├── main.go                    入口（直接启动 Web 服务器）
├── cmd/
│   ├── serve.go               根命令：Web 服务器 + WebSocket + 路由 + TLS
│   ├── auth.go                登录认证（Cookie + HMAC）
│   ├── files.go               文件浏览 / 编辑 + 工作目录 API
│   └── tls.go                 自签名证书
├── internal/
│   ├── ai/                    LLMClient 接口 + 工厂 + openai/claude/google + ListModels
│   ├── agent/                 主 Agent、工具、子代理 DAG、压缩、MCP、Skills
│   ├── session/               多会话 + 实时落盘
│   ├── settings/              设置 + ~/.licode 数据目录
│   ├── procutil/              可执行文件解析
│   ├── websocket/             Hub + 事件协议
│   ├── version/               0.0.0.x 版本计数
│   └── web/                   go:embed Nuxt 前端产物与 CA 证书
├── cordis/                    Cordis-Go 运行时（微内核）
├── plugins/                   内置插件（编译进二进制）
├── nuxtweb/                   Nuxt 3 + Vue 前端源码
└── docs/                      文档
```



---

文档

· 配置与数据目录
· 多厂商与模型
· 子代理系统
· Web 界面与 API
· 插件系统方案
· 插件开发指南
· 开发与构建
· FAQ

---

版本号

版本号采用 0.0.0.x 格式，跟随 ~/.licode/version 计数（百进制进位）。发行构建会通过 -X licode/internal/version.Version= 注入版本。

---

许可

请参阅仓库中的 LICENSE 文件。
