# WebSocket 协议

前端（`web/`）与后端通过 `/ws` 通信。协议类型定义在前端 `web/src/api/protocol.ts`，
后端定义在 `internal/websocket/websocket.go`。**两处必须同步修改**。

## 客户端 -> 服务端

| type | 载荷 | 说明 |
| --- | --- | --- |
| `message` | `{content, attachments?}` | 发送用户消息，触发一轮 Agent 运行 |
| `ping` | - | 应用层保活（前端 25s 一次，后端读超时 75s） |
| `settings_get` / `settings_set` | `{settings?}` | 读取/写入设置 |
| `ask_reply` | `{askId, askApprove, askAlways}` | 工具审批回执；`askAlways=true` 表示「始终允许」，仅当前会话生效 |
| `interrupt` | - | 中止当前生成 |
| `sessions_get` | - | 拉取会话列表 |
| `session_new` | - | 新建会话 |
| `session_switch` | `{sessionId}` | 切换当前会话 |
| `session_rename` | `{sessionId, content}` | 重命名 |
| `session_delete` | `{sessionId}` | 删除（移入 `.trash/`） |
| `session_branch` | `{sessionId, index}` | 从某条消息分叉 |
| `session_history` | `{sessionId}` | 请求完整历史 |
| `session_reorder` | `{order: [id...]}` | 按给定顺序重排；未列出的 id 由服务端追加到末尾 |
| `session_pin` | `{sessionId, pinned}` | 置顶/取消置顶 |

## 服务端 -> 客户端

| type | 载荷 | 说明 |
| --- | --- | --- |
| `delta` | `{content, sessionId}` | 助手输出增量，前端追加到末尾的助手消息 |
| `tool_start` | `{toolName, toolArgs, sessionId}` | 工具开始执行，新建一张 running 卡片 |
| `tool_done` | `{toolName, toolOut, sessionId}` | 工具结束，回填最后一条同名 running 卡片 |
| `ask` | `{toolName, toolArgs, askId, sessionId}` | 请求审批，前端展示审批卡片 |
| `done` | `{sessionId}` | 本轮结束 |
| `error` | `{error, sessionId}` | 出错 |
| `status` | `{content}` | 状态提示（如「思考中」） |
| `reasoning` | `{content}` | 推理过程（骨架中暂未呈现） |
| `sessions` | `{sessions: [{id,title,count,pinned?}], sessionId}` | 会话列表全量推送 |
| `history` | `{messages: [...], sessionId}` | 历史消息回放 |
| `settings` | `{settings}` | 设置全量推送 |
| `stats` | `{stats}` | 用量统计 |

## 关键约束

- **置顶与会话顺序由服务端持有**。前端 `session_reorder` / `session_pin` 只发意图，
  顺序一律以服务端回推的 `sessions` 事件为准。前端可做乐观更新以避免拖拽回弹，
  但不得把本地顺序当作真源。
- **`ask` 与 `ask_reply` 通过 `askId` 配对**。前端回执时必须原样带回 `askId`。
  一轮结束时（`done` / `error`）服务端会清理未决审批，前端应同步作废本地待审批项，
  否则会展示永远得不到响应的审批卡片。
- **同源校验**：`/ws` 校验 `Origin` 与 Host（端口敏感）。前端必须用
  `window.location.host` 推导连接地址，不要硬编码 host，否则反代/局域网访问会被拒。
- **会话事件按 `sessionId` 过滤**：切换会话后到达的旧会话事件必须丢弃，否则会串台。

## 本地验证

无外网/无 API key 时，可用内置 mock 端点跑通全链路（流式 + 工具 + 审批）：

```bash
node scripts/mock-llm.mjs        # 监听 127.0.0.1:11434/v1
# 然后在设置里指向它：base_url=http://127.0.0.1:11434/v1, api_key=mock, model=mock-agent
```
