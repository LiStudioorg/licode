# licode 开发与构建

## 环境

- Go ≥ 1.22（go.mod 声明）
- 纯 Go 依赖：fsnotify、gorilla/websocket、cobra、BurntSushi/toml；无 CGO

## 编译

```bash
go build ./...        # 编译检查
go test ./...         # 全部测试
./build.sh            # 9 平台交叉编译（CGO_ENABLED=0，产物 build/licode-*）
```

## 目录结构

```
├── main.go             入口（直接启动 Web 服务器）
├── cmd/
│   ├── serve.go        根命令：Web 服务器 + WebSocket + 路由 + TLS
│   ├── auth.go         登录认证（Cookie + HMAC）
│   ├── files.go        文件浏览/编辑 + 工作目录 API
│   └── tls.go          自签名证书
├── internal/
│   ├── ai/             LLMClient 接口 + 工厂 + openai/claude/google + ListModels
│   ├── agent/          主 Agent、工具、子代理 DAG、压缩、MCP、Skills
│   ├── session/        多会话 + 实时落盘
│   ├── settings/       设置 + ~/.licode 数据目录
│   ├── websocket/      Hub + 事件协议
│   ├── version/        0.0.0.x 版本计数
│   └── web/            go:embed Nuxt 前端产物与 CA 证书
└── docs/               文档
```

## 测试

- `internal/ai`：OpenAI SSE 流式与工具调用解析（mock 服务器）
- `internal/agent`：工具循环、会话截断、子代理 DAG 顺序/环检测

## 设计要点

- 一切 UI 文案简体中文默认；不依赖 CGO（CGO_ENABLED=0）
- 系统提示词读取 `~/.licode/system-prompt.md`（可直接改）
- 版本号 0.0.0.x 跟随 `~/.licode/version` 计数（百进制进位）