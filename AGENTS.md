# AGENTS.md

本项目为 opencode 及其他 AI 编码助手自动读取的项目说明，所有改动请先阅读本文件。

## 项目概览

- Licode 是一个单二进制 Web 应用：Go 后端提供 HTTP/WebSocket/Agent(LLM) 全栈服务，前端为 **Vite + Vue 3 + Pinia + Tailwind v4** 编译后的静态资源（SPA），通过 `go:embed all:dist`（`internal/web/`）嵌入二进制。
- 入口 `main.go` → `cmd.Execute()` 直接启动 Web 服务器（无子命令）。
- 核心模块：`cmd/`(HTTP 路由层)、`internal/agent/`(LLM Agent 与工具)、`internal/websocket/`(实时会话)、`internal/ai/`(LLM 客户端抽象)、`internal/settings/`、`internal/backup/`。
- 运行数据保存在 `~/.licode/`（config.json、sessions/、session.key 等）。

## 前端（web/）

- 技术栈：Vite 7 + Vue 3 + vue-router 4 + Pinia 3 + Tailwind CSS v4 + TypeScript（strict）。纯静态构建，产物为 `web/dist`。
- **无任何 auto-import 插件**：`vite.config.ts` 只有 `@vitejs/plugin-vue` 与 `@tailwindcss/vite`，组件不做全局注册、composable 不自动注入。
- 主题 token：Tailwind v4 的 `@theme` 变量（`src/styles/main.css`）。Tailwind 工具类编译成 `var(--color-*)` 引用，因此运行期覆写 `:root` 同名变量即可换肤。深色为默认，浅色是 html 上的 `.light` 类。
- 图标：`src/utils/icons.ts` 里的 16×16 SVG path 字符串（`<AppIcon name="send" />`），**离线自包含**，不请求任何图标服务。
- 目录：`src/components/`（含 `ui/` 基础组件）、`src/views/`（路由页）、`src/stores/`（Pinia）、`src/composables/`、`src/api/`（WS 协议 + REST）、`src/utils/`、`src/types/`。
- 路由对应关系：`/` 对话、`/login` 登录、`/files` 文件、`/info` 信息、`/settings` 设置、`/tools`（重定向到 `/settings`）。
- 主题：`useTheme` 管 `.light` 类、强调色（`--color-accent*`）、页面背景（`--color-canvas` 及派生的 surface/elevated/sunken），全部持久化到 localStorage。

### 前端构建（需用户显式授权才可跑 node）

```bash
cd web && npm install               # 首次/依赖变更时
cd web && npm run build             # vue-tsc -b && vite build，产出 web/dist
LICODE_FRONTEND_BUILD=1 bash scripts/sync-frontend.sh   # 同步到 internal/web/dist（go:embed）
```

- **默认不进构建**：`build.sh` / `Makefile` 调用的 `scripts/sync-frontend.sh` 在未设 `LICODE_FRONTEND_BUILD=1` 时直接跳过，因此 `go build` 与发布流程始终是纯 Go，不依赖 node。
- 只有用户明确授权重建前端时才设 `LICODE_FRONTEND_BUILD=1`。改完 `web/src/**` 若不同步，二进制里的前端不会变化。
- **不要手改 `internal/web/dist/`**：那是 `web/dist` 的同步副本，会被下次同步整体覆盖。

### 前端易错点（血泪教训）

- **Vue API 必须显式 import**：本项目**没有** auto-import。`ref/computed/watch/onMounted/nextTick`、每个子组件、每个 composable 一律要自己 import，漏了构建期可能静默通过、运行期报 `xxx is not defined`。`vue-tsc -b` 会抓到大部分（`noUnusedLocals` 也会抓多余 import）。
- **模板里的 `document` / `Event` 等全局可能被 setup 绑定遮蔽**：Volar 会以 setup 作用域解析模板表达式，若 store 变量名与全局同名（如 `session`），`document`/`Event` 会报「不存在于该类型」。把 DOM 操作提成 script 里的方法，别在模板内联。
- **`<script setup>` 的 ref 在模板里自动解包，但嵌套对象的 ref 不会**：`const theme = useTheme()` 后模板写 `theme.isDark.value` 会拿到 ref 对象而非布尔值——要么解构 `const { isDark } = useTheme()`，要么在模板显式 `.value`（Volar 会报后者类型错）。
- **外观设置驱动的是 `--color-*` token**：强调色写 `--color-accent`（完整颜色值如 hex；Tailwind v4 不是 "R G B" 三段式），背景写 `--color-canvas` 并派生 `--color-surface/elevated/sunken`。写自定义变量（`--accent` 之类）不会有任何效果。
- **首帧主题内联脚本与 CSP 哈希联动**：`web/index.html` 里的防闪白内联脚本会被 `internal/web/csp.go` 的 `ScriptSrcCSP()` 在启动时算出 sha256 并写进 `script-src`。改这段内联脚本**必须重新构建并同步产物**，否则哈希失配、浏览器静默拦截该脚本。
- **批量 sed 改 SFC 属性极易制造重复属性/断绑定**（`class` 写两遍、`:label` 变 `label`、`v-else` 失联）。改完必须跑 `npx vue-tsc -b`。

## 硬性约束

- **前端产物预生成入库**：`internal/web/dist/` 的静态文件已提交，未获用户授权时**不要**运行 npm/vite 构建（见上「前端构建」）。后端改动是默认可交付路径。
- **不引入新的运行时网络依赖**：前端产物必须可离线自包含（无外网 CDN）。
- 不使用全局可变状态（历史教训 `internal/agent/mcp.go` 已重写为无状态实现）。

## 构建与验证命令

```bash
go build ./...
go vet ./...
go test ./...        # 全绿；无 node 相关依赖
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