# AGENTS.md

本项目为 opencode 及其他 AI 编码助手自动读取的项目说明，所有改动请先阅读本文件。

## 项目概览

- Licode 是一个单二进制 Web 应用：Go 后端提供 HTTP/WebSocket/Agent(LLM) 全栈服务，前端为 Nuxt 编译后的静态资源，通过 `go:embed internal/web/nuxt` 嵌入二进制。
- 入口 `main.go` → `cmd.Execute()` 直接启动 Web 服务器（无子命令）。
- 核心模块：`cmd/`(HTTP 路由层)、`internal/agent/`(LLM Agent 与工具)、`internal/websocket/`(实时会话)、`internal/ai/`(LLM 客户端抽象)、`internal/settings/`、`internal/search/`(自建搜索)、`internal/backup/`。
- 运行数据保存在 `~/.licode/`（config.json、sessions/、session.key 等）。

## 硬性约束

- **绝对禁止 nodejs**：不得在构建、开发或发布流程中引入/要求 node/npm/nuxt。前端产物 `internal/web/nuxt/` 的静态文件是预生成并已入库的，修改 `nuxtweb/` 下的 .vue 源码后**不要**重新运行 npm/build（除非用户明确授权），否则改动无法进入二进制。后端改动才是可交付路径。
- 不使用全局可变状态（历史教训 `internal/agent/mcp.go` 已重写为无状态实现）。
- 修改 `internal/search/` 的 URL 校验逻辑时注意 `service_test.go` 使用 127.0.0.1 的 httptest，私网拦截有测试逃生开关 `blockPrivateHosts`。

## 构建与验证命令

```bash
go build ./...
go vet ./...
go test ./...        # 注意：internal/agent 的 TestAgentToolLoop 为既有失败，与本次改动无关
CGO_ENABLED=0 ./build.sh   # 9 平台静态编译，产物在 build/，全程纯 Go（无 node）
```

## Git 提交规则

- 严禁在 commit message 中添加任何形式的 AI 署名或 Co-Authored-By 标签。
- 按导入顺序分块列出变更要点，说明「为什么」优先于「做了什么」。
- 提交前用 `git status` / `git diff` 检查并只暂存预期文件，遵守仓库既有提交风格（中文描述、带类型前缀如 `feat:`/`fix:`）。
- 用户未明确要求时不要自行 commit、push 或打 tag。

## 发布流程（用户授权后）

1. `CGO_ENABLED=0 ./build.sh`（确认无 node 提示、9/9 成功）
2. 显式 `git add` 相关文件 → commit（遵守 Git 提交规则）
3. `git tag v0.0.0.x` → 推 GitHub（`origin` push 为 github）与 Gitee（Gitee 推送需在 URL 中使用用户提供的一次性 token，仓库内存储的 token 会过期）
4. `gh release create v0.0.0.x --repo li63050a6/licode` + 上传 `build/` 下 9 个资产
5. 安装到 `~/.local/bin/licode`