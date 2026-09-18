# 03 · 后端：REST API 参考

> 所有端点返回 JSON（`/health`、`/api/export`、`/api/download` 例外）。错误统一为 `{"error":"..."}`（**中文可读文本**，前端直接展示）。
> 除标注「无需认证」外均需登录（未登录时若请求头带 `Accept: application/json` 返回 **401 纯文本**，否则 `302 → /login`）。
> 请求体上限：文件保存/删除等 ≤1MB；上传 ≤256MB；导入 ≤64MB。

## 1. 认证 / 元信息

### `GET /health`（无需认证）
```
200 {"status":"ok"}
```

### `GET /ready`（无需认证）
```
200 {"status":"ready"}
503 {"status":"shutting_down"}
503 {"status":"not_ready","problems":["llm_unreachable","docker_unavailable"]}
```
探测项：base_url TCP 拨号（15s 缓存）、docker info。

### `GET /api/auth`（无需认证）
```
200 {"enabled":false,"username":"licode","default_username":"licode"}
```
前端用它判断是否显示登录页。

### `GET /api/version`
```
200 {"version":"0.0.0.2","counter":2}
```

## 2. 文件与工作目录（`cmd/files.go`）

### `GET /api/files?path=` 列目录
- `path` 绝对路径或工作目录相对路径；空 = 工作目录；`.` = 工作目录。
```
200 {"root":"D:\\code\\licode","path":"D:\\code\\licode",
     "entries":[{"name":"cmd","path":"D:\\code\\licode\\cmd","isDir":true,"size":0},
                {"name":"go.mod","path":"D:\\code\\licode\\go.mod","isDir":false,"size":1207}]}
```
- 隐藏文件（`.` 开头）不显示；目录排在文件前。
- `root` 是工作目录，`path` 是实际列出的目录（可越出 root 全盘浏览）。

### `GET /api/file?path=` 读文件
```
200 {"path":"绝对路径","content":"..."}
```

### `POST /api/file` 写文件（≤1MB）
```json
{ "path": "相对或绝对路径", "content": "文本内容" }
```
```
200 {"ok":true,"path":"绝对路径"}
```
- 自动创建父目录。

### `POST /api/mkdir` 建目录
```json
{ "path": "..." }
```
```
200 {"ok":true,"path":"绝对路径"}
```

### `POST /api/delete` 删除
```json
{ "path": "...", "recursive": false }
```
```
200 {"ok":true}
```
- 目录非空且未带 `recursive:true` → **409** `{"error":"目录非空…"}`（前端据此二次确认递归删除）。

### `POST /api/chmod` 修改权限
```json
{ "path": "...", "mode": "644" }
```
```
200 {"ok":true,"path":"绝对路径","mode":"644"}
```

### `POST /api/chown` 修改属主
```json
{ "path": "...", "owner": "1000:1000" }
```
```
200 {"ok":true,"path":"绝对路径","uid":1000,"gid":1000}
```
- `-1` 表示该项保持不变。

### `POST /api/upload` 上传（multipart，≤256MB）
- 表单字段：`file`（**单个**文件）、`dir`（目标目录，绝对路径或相对工作目录，**缺省 = 工作目录**）。
- 保留原始文件名（去路径、净化非法字符）；重名自动 `_1/_2` 去重。
- 返回 `path` 为**绝对路径**；**没有 `url` 字段**（旧版 `/uploads/*` 路由也不存在，别用）。
```
200 {"ok":true,"path":"D:\\code\\licode\\nuxtweb\\_x.txt"}
400 {"error":"目录无效"|"目标目录不存在"|"文件过大或格式错误"}
```

### `GET /api/download?path=` 下载
- 文件 → 附件直链（`Content-Disposition: attachment`，兼容中文文件名）。
- 目录 → 实时打包 zip 流式下载（文件名 `<目录名>.zip`）。

### `GET/POST /api/workspace` 工作目录
```
GET 200 {"root":"D:\\code\\licode"}
POST {"path":"D:\\xxx"} → 200 {"ok":true,"root":"D:\\xxx"}
```

## 3. 备份（`cmd/backup.go`）

### `GET /api/export`
- zip 下载，`Content-Disposition: attachment; filename="licode-backup.zip"`。

### `POST /api/import`（原始 zip 字节，≤64MB）
- 请求体直接是 zip 二进制（**不是 FormData**），`Accept: application/json`。
```
200 {"ok":true}
200 {"ok":false,"error":"..."}
```

## 4. 工具管理（`cmd/tools_api.go`）

### `GET /api/tools`
- 返回全部工具（内置、子代理、MCP、外部命令、技能）及生效权限与 MCP 服务器配置。
- MCP 工具枚举 8s 超时，连接后立即释放子进程；连接失败返回空列表 + `mcp_error`。
```
200 {"tools":[{"name":"Read","description":"…","source":"builtin","removable":false,"rule":"allow"}],"mcp_servers":[],"mcp_error":""}
```
- `source`：`builtin|subagent|mcp|external|skill`；`rule`：`allow|ask|deny`。

### `POST /api/tools/rule`
```json
{ "name": "Shell", "rule": "ask" }
```
```
200 {"ok":true}
400 {"error":"规则取值无效（应为 允许/询问/禁止）"}
```

### `POST /api/tools/delete`
```json
{ "type": "mcp", "name": "git" }
```
- `type`：`mcp`（删除服务器配置并清理其工具规则）、`external`（删 `~/.licode/tools/*.json`）、`skill`（删技能 md）。
- 文件型删除严格限定在对应目录内。
```
200 {"ok":true}
404 {"error":"未找到该 MCP 服务器"}
```

## 5. 模型列表

### `GET /api/models?type=&base=&provider=`（20s 超时）
- 使用**当前激活厂商的 api_key**（来自 settings 快照）；query 仅覆盖 `type`/`base_url`/`provider`。
- `type` 支持 `openai`（含 Ollama 兼容）、`google`、`claude`（无列表接口返回空）、以及 `ollama`/`gemini`（会被归一）。
- 各厂商真实行为：openai → `GET {base}/models`；google → `GET {base}/v1beta/models?key=`（仅保留含 gemini 的名字）；claude → 空。
```
200 {"provider":"openai","type":"openai","models":["gpt-4o-mini",...]}
502 {"error":"..."}
```
- **前端注意**：给「新厂商」取模型前必须先把该厂商设为激活（`settings_set`），否则用的是旧激活厂商的 key（见 06）。

## 6. 通用约定
- 错误一律 `{"error":"中文说明"}` + 4xx/5xx。
- 未登录：带 `Accept: application/json` → 401 纯文本「401 未登录」；否则 302 → `/login`。前端 `useApi` 始终带 `Accept: application/json`。
- `GET /`：需要登录时返回 Nuxt SPA；`/settings`、`/tools` 返回各自预渲染页面。
