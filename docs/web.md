# licode Web 界面与 API

## 界面

浏览器打开 `http://<host>:<port>` 即用，手机/电脑自适应。页面资源全部内嵌在二进制里，无需外网/CDN。

- 对话：左侧会话列表（新建/切换/重命名/导出/删除），中间流式对话与工具调用，右侧信息/文件面板。
- 设置（`/settings`）：基础、AI 厂商、高级、MCP、外观五个分区；DNS 支持「跟随系统 / 自定义服务器 / 系统命令」三种方式。
- 工具管理（设置页内页签）：内置、子代理、MCP、外部命令、技能工具的统一管理；权限「允许 / 询问 / 禁止」，可删除 MCP 配置与文件型工具。
- 外观：浅色/深色 + 液态玻璃皮肤（毛玻璃 + 渐变背景）。

## 登录

`--password` 或 `LICODE_PASSWORD` 设置密码后启用登录页（默认用户名 `licode`）。会话 Cookie（HMAC 签名）保护网页与 WebSocket；未启用登录时页面顶部有提醒横幅。

## 主要 API

| 分类 | 路径 |
| --- | --- |
| 文件 | `/api/files`、`/api/file`、`/api/mkdir`、`/api/delete`、`/api/chmod`、`/api/chown`、`/api/upload`、`/api/download`、`/api/workspace` |
| 工具管理 | `/api/tools`、`/api/tools/rule`、`/api/tools/delete` |
| 其他 | `/api/auth`、`/api/version`、`/api/models`、`/api/export`、`/api/import`、`/api/ca`、`/api/shells` |

文件管理器语义：登录后 `/api/*` 可访问任意绝对路径；AI 工具层限制在工作目录内，越界访问需要用户确认。

## 工具权限

「允许」直接执行、「询问」每次弹窗确认（可本对话始终允许）、「禁止」模型不可调用。未知工具（MCP/外部命令/技能）默认「询问」。

## WebSocket 协议

`/ws`；完整消息/事件列表见 `nuxtweb/docs/04-后端-WebSocket协议.md`。
