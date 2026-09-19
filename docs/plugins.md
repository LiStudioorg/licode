# licode 插件开发指南（进程插件）

licode 的插件是**独立进程**：任何语言都能写，通过标准输入/输出上的 JSON-RPC 与宿主通信。
插件可以贡献工具、斜杠命令、提示词、设置界面与只读面板。

> 进程插件由操作系统直接运行，宿主无法强制沙箱。启用时会展示插件声明的权限，请只启用可信插件。

## 1. 目录结构

把插件放在以下任一目录（每个插件一个子目录）：

- `~/.licode/plugins/<id>/`（用户级）
- `<项目>/.licode/plugins/<id>/`（项目级）

```
~/.licode/plugins/hello/
├── plugin.json     # 清单（必需）
└── main.py         # 任意语言的入口程序
```

宿主会监控这些目录：新增/修改插件会**热加载**（版本号变化时自动重启进程），删除目录会停止并卸载插件。

## 2. 清单 plugin.json

```json
{
  "id": "hello",
  "name": "问候插件",
  "version": "0.1.0",
  "apiVersion": 1,
  "description": "演示插件",
  "author": "you",
  "entry": "python3",
  "args": ["main.py"],
  "env": { "MY_VAR": "value" },
  "capabilities": ["tools", "commands", "panels", "settings", "hooks"],
  "permissions": { "fs": ["~/docs"], "net": ["example.com"], "shell": false, "env": false },
  "prompt": "你可以调用 hello 工具向用户问好。",
  "contributes": {
    "tools": [
      { "name": "hello", "description": "打招呼",
        "schema": { "type": "object", "properties": { "name": { "type": "string" } }, "required": ["name"] } }
    ],
    "commands": [ { "name": "hello", "description": "示例命令" } ],
    "panels": [ { "id": "status", "title": "状态面板" } ],
    "settings": {
      "type": "object",
      "properties": {
        "greeting": { "type": "string", "title": "问候语", "default": "你好" },
        "loud": { "type": "boolean", "title": "大写输出" }
      }
    }
  }
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `id` | 小写字母/数字/`_`/`-`，2-41 字符，同时是目录名与工具前缀 |
| `entry` / `args` | 启动命令（在插件目录下执行；`entry` 用绝对路径或 PATH 中的名字） |
| `env` | 追加给插件进程的环境变量 |
| `permissions` | 权限声明，仅用于启用确认（`env: true` 时继承宿主全部环境变量，否则只给 PATH/HOME/LANG） |
| `prompt` | 追加到系统提示词（插件运行中时生效） |
| `contributes.tools` | 注册为模型工具，名称 `plugin__<id>__<name>`，默认权限「询问」 |
| `contributes.commands` | 对话里输入 `/<name>`（或 `/<id>:<name>`）直接执行，不经过模型 |
| `contributes.panels` | 设置页「插件」中渲染的只读面板 |
| `contributes.settings` | JSON Schema，宿主自动渲染设置表单，保存后通知插件 |

## 3. 通信协议（stdio JSON-RPC 2.0）

宿主写入的帧使用 `Content-Length` 头（与 LSP/MCP 相同）：

```
Content-Length: 57\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
```

插件返回时同样使用 `Content-Length` 帧；也支持一行一个 JSON（NDJSON）以简化实现。

### 宿主 → 插件（请求，需返回 result）

| 方法 | 参数 | 返回 |
| --- | --- | --- |
| `initialize` | `{apiVersion, plugin:{id,version}, settings}` | `{"ok":true}` |
| `tools/call` | `{name, arguments}` | `{"content":"..."}` 或 `"文本"` |
| `command/run` | `{name, args}` | `{"content":"..."}` |
| `panel/render` | `{panel}` | `{"type":"markdown\|table\|keyvalue\|status", ...}` |
| `hook/run` | `{event, payload}` | `{"action":"continue"}` / `{"action":"modify","content":"..."}` / `{"action":"deny","content":"原因"}` |

`panel/render` 支持四种展示类型：

```json
{ "type": "markdown", "content": "# 标题" }
{ "type": "table", "columns": ["列1"], "rows": [["值"]] }
{ "type": "keyvalue", "items": [ { "key": "k", "value": "v" } ] }
{ "type": "status", "text": "一切正常" }
```

### 宿主 → 插件（通知，无需响应）

| 方法 | 参数 |
| --- | --- |
| `shutdown` | 无（插件退出前应返回并结束进程） |
| `settings/update` | `{settings}` |

### 插件 → 宿主（通知）

```json
{ "jsonrpc":"2.0", "method":"log", "params": { "level":"info", "message":"..." } }
```

日志会出现在设置页「插件」的日志区（最多保留 200 条）。

### 钩子（capabilities 含 `hooks`）

| 事件 | payload | 可返回 |
| --- | --- | --- |
| `user_message` | `{content}` | `modify` + `content` 改写用户消息 |
| `before_tool` | `{tool, args}` | `deny` 拒绝执行；`modify` + `payload`（新参数 JSON） |
| `after_tool` | `{tool, args, output}` | `modify` + `content` 改写工具输出 |
| `done` | `{}` | 忽略 |

每个钩子调用超时 5 秒，失败只记录日志、不阻断主流程。

## 4. 最小示例（Python）

```python
#!/usr/bin/env python3
import json, sys

def read_msg():
    line = sys.stdin.buffer.readline()
    if not line:
        return None
    if line.lstrip().startswith(b"{"):
        return json.loads(line)
    headers, l = {}, line
    while l not in (b"\r\n", b"\n", b""):
        k, _, v = l.decode().partition(":")
        headers[k.strip().lower()] = v.strip()
        l = sys.stdin.buffer.readline()
    return json.loads(sys.stdin.buffer.read(int(headers.get("content-length", 0))))

def send(obj):
    b = json.dumps(obj).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\r\n\r\n" % len(b) + b)
    sys.stdout.buffer.flush()

while (msg := read_msg()) is not None:
    m, mid = msg.get("method"), msg.get("id")
    if m == "shutdown":
        break
    if m == "tools/call":
        args = (msg.get("params") or {}).get("arguments") or {}
        send({"jsonrpc": "2.0", "id": mid, "result": {"content": "你好，" + args.get("name", "世界")}})
    elif mid is not None:
        send({"jsonrpc": "2.0", "id": mid, "result": {}})
```

## 5. 安装、启用与管理

- 安装：设置 → 插件 → 「安装插件（zip）」，或直接把目录放进插件目录
- 新插件默认**未启用**；点击「启用」会弹出权限确认，确认后进程才会启动
- 插件工具默认权限「询问」，可在「工具管理」页改为允许/禁止
- 支持重载（重新读清单并重启进程）、停用、删除（目录移入 `.trash` 可恢复）
- 状态保存在 `~/.licode/plugins.state.json`（启停、权限确认、插件设置）
