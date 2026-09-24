package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"licode/internal/agent"
	"licode/internal/ai"
	"licode/internal/rag"
	"licode/internal/session"
	"licode/internal/settings"
	"licode/internal/version"
	"licode/internal/web"
	"licode/internal/websocket"
)

// ServeOptions holds resolved configuration for the serve command.
type ServeOptions struct {
	Host        string
	Port        int
	NoSubAgents bool
	Username    string
	Password    string
	HTTPS       bool
	TLSCert     string
	TLSKey      string
	ConfigPath  string
}

// NewRootCommand 返回根命令：licode 直接运行即启动 Web 服务器。
func NewRootCommand() *cobra.Command {
	root := newServeCmd()
	root.Use = "licode"
	root.Short = "AI 编程助手（Web 服务器，licode 直接运行即启动）"
	return root
}

func newServeCmd() *cobra.Command {
	opts := &ServeOptions{}
	c := &cobra.Command{
		Use:   "web",
		Short: "AI 编程助手（Web 界面）",
		Long: `licode 直接运行即启动 Web 服务器。

启动 Web 服务器，浏览器访问 http://<host>:<port> 即可使用，例如：
    licode --host 0.0.0.0 --port 8080
    licode --password 你的密码                （设置后启用登录，默认用户名 licode）

浏览器访问 http://<host>:<port> 即可使用，支持手机/电脑。
所有 AI 推理都在本服务器执行。设置可在网页端实时修改并写回
~/.licode/config.toml，无需重启。`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 配置文件：默认 ~/.licode/config.toml；用 -c 指定其他路径
			cfgPath := opts.ConfigPath
			if cfgPath == "" {
				cfgPath = settings.ConfigTOMLPath()
			}
			// 先建目录再读写配置：全新环境下 ~/.licode 尚不存在，
			// 若先生成 config.toml 会因父目录缺失直接报 no such file or directory。
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
				return fmt.Errorf("创建配置目录 %s 失败: %w", filepath.Dir(cfgPath), err)
			}
			cfg, err := settings.LoadTOML(cfgPath)
			if os.IsNotExist(err) {
				cfg = settings.DefaultTOML()
				if gerr := settings.GenerateTOML(cfgPath, cfg); gerr != nil {
					return fmt.Errorf("生成配置文件 %s 失败: %w", cfgPath, gerr)
				}
				log.Printf("已生成配置文件 %s", cfgPath)
			} else if err != nil {
				return fmt.Errorf("加载配置文件 %s: %w", cfgPath, err)
			}
			if !cmd.Flags().Changed("host") && cfg.Server.Host != "" {
				opts.Host = cfg.Server.Host
			}
			if !cmd.Flags().Changed("port") && cfg.Server.Port != 0 {
				opts.Port = cfg.Server.Port
			}
			if !cmd.Flags().Changed("username") && cfg.Server.Username != "" {
				opts.Username = cfg.Server.Username
			}
			if !cmd.Flags().Changed("password") && cfg.Server.Password != "" {
				opts.Password = cfg.Server.Password
			}
			if !cmd.Flags().Changed("https") && cfg.Server.HTTPS {
				opts.HTTPS = true
			}
			if !cmd.Flags().Changed("tls-cert") && cfg.Server.TLSCert != "" {
				opts.TLSCert = cfg.Server.TLSCert
			}
			if !cmd.Flags().Changed("tls-key") && cfg.Server.TLSKey != "" {
				opts.TLSKey = cfg.Server.TLSKey
			}
			return runServe(opts)
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Host, "host", "127.0.0.1", "监听主机（默认 127.0.0.1，局域网/手机访问用 0.0.0.0）")
	f.IntVar(&opts.Port, "port", 8080, "监听端口")
	f.BoolVar(&opts.NoSubAgents, "no-subagents", false, "禁用子代理编排")
	f.StringVar(&opts.Username, "username", "", "登录用户名（默认 licode；环境变量 LICODE_USERNAME）")
	f.StringVar(&opts.Password, "password", "", "登录密码（环境变量 LICODE_PASSWORD）；未设置则不启用登录")
	f.BoolVar(&opts.HTTPS, "https", false, "启用 HTTPS（未指定证书时自动生成自签名证书）")
	f.StringVar(&opts.TLSCert, "tls-cert", "", "TLS 证书文件路径（cert.pem）")
	f.StringVar(&opts.TLSKey, "tls-key", "", "TLS 私钥文件路径（key.pem）")
	f.StringVarP(&opts.ConfigPath, "config", "c", "", "配置文件路径（默认 ~/.licode/config.toml）")
	return c
}

// listenAddr 计算监听地址 host:port。
func listenAddr(opts *ServeOptions) string {
	return fmt.Sprintf("%s:%d", opts.Host, opts.Port)
}

// serverState 持有可变的全局设置与当前客户端。
type serverState struct {
	mu           sync.RWMutex
	settings     settings.Settings
	client       ai.LLMClient
	shuttingDown bool       // 收到关停信号后置位，拒绝新连接
	rag          *rag.Index // 特性5：项目源码轻量 RAG 索引（懒构建）
}

// connState 保存每个连接独立的会话（多对话）与待确认的工具调用。
// busy/interruptCancel 按会话 ID 独立跟踪：同一连接下多个会话可并行处理，
// 切换会话后发消息不会被其他会话的运行状态误锁。
type connState struct {
	mu              sync.Mutex
	sessions        *session.Manager
	pending         map[string]chan bool
	askTool         map[string]string // askID -> 工具名
	askSeq          atomic.Int64
	busy            map[string]bool               // sessionID -> 是否处理中
	interruptCancel map[string]context.CancelFunc // sessionID -> 可取消的运行上下文
}

func newConnState(sessionsDir string) *connState {
	return &connState{
		sessions:        session.NewManager(sessionsDir, true),
		pending:         map[string]chan bool{},
		askTool:         map[string]string{},
		busy:            map[string]bool{},
		interruptCancel: map[string]context.CancelFunc{},
	}
}

func runServe(opts *ServeOptions) error {
	// 首次使用自动生成 ~/.licode 数据目录，并启用日志文件
	if err := settings.EnsureDirs(); err != nil {
		return fmt.Errorf("初始化数据目录失败: %w", err)
	}
	if lf, err := settings.LogFile(); err == nil {
		defer lf.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, lf))
	}

	// 版本计数递增（0.0.0.0 → … → 0.0.0.100 → 0.0.1.0）
	runVersion := version.Bump()
	log.Printf("licode 版本 %s", runVersion)

	// 特性6：工具热加载（fsnotify 监视 ~/.licode/tools/，动态注册/卸载外部命令工具）
	if toolClose, terr := agent.StartExternalToolWatcher(settings.ToolsDir()); terr == nil {
		defer toolClose()
		log.Printf("外部工具热加载已启动: %s", settings.ToolsDir())
	}

	st := &serverState{}
	st.settings = settings.Defaults()
	st.settings.ApplyFlags(opts.NoSubAgents)

	client, err := st.settings.NewClient()
	if err != nil {
		return err
	}
	st.client = client

	hub := websocket.NewHub()
	hub.OnConnect(func(ctx context.Context, c *websocket.Client) {
		// 关停期间拒绝新连接
		st.mu.RLock()
		shuttingDown := st.shuttingDown
		st.mu.RUnlock()
		if shuttingDown {
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtError, Error: "服务正在关停，请稍后再试"})
			return
		}
		log.Printf("客户端已连接（当前 %d 个）", hub.Count())
		cs := newConnState(settings.SessionsDir())

		c.OnUserMessage(func(ctx context.Context, msg websocket.ClientMessage) {
			switch msg.Type {
			case websocket.TypeSettingsGet:
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSettings, Settings: st.settings.Snapshot().Masked(),
				})

			case websocket.TypeSettingsSet:
				if err := applyServerSettings(st, msg); err != nil {
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtError, Error: err.Error()})
				} else {
					// 设置修改同步写回配置文件
					_ = st.settings.Save("")
				}
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSettings, Settings: st.settings.Snapshot().Masked(),
				})

			case websocket.TypeSessionsGet:
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSessions, Sessions: cs.sessions.List(), SessionID: cs.sessions.CurrentID(),
				})

			case websocket.TypeSessionNew:
				cs.sessions.New()
				_ = cs.sessions.SaveAll()
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSessions, Sessions: cs.sessions.List(), SessionID: cs.sessions.CurrentID(),
				})

			case websocket.TypeSessionSwitch:
				if cs.sessions.SetCurrent(msg.SessionID) {
					_ = cs.sessions.SaveAll()
					c.SendEvent(websocket.ServerEvent{
						Type: websocket.EvtSessions, Sessions: cs.sessions.List(), SessionID: cs.sessions.CurrentID(),
					})
				}

			case websocket.TypeSessionHistory:
				msgs := cs.sessions.Messages(msg.SessionID)
				if msgs == nil {
					msgs = []ai.Message{}
				}
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtHistory, SessionID: msg.SessionID, Messages: msgs,
				})

			case websocket.TypeSessionRename:
				cs.sessions.Rename(msg.SessionID, msg.Content)
				_ = cs.sessions.SaveAll()
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSessions, Sessions: cs.sessions.List(), SessionID: cs.sessions.CurrentID(),
				})

			case websocket.TypeSessionDelete:
				log.Printf("删除会话: %s（已移入 sessions/.trash/）", msg.SessionID)
				cs.sessions.Delete(msg.SessionID)
				_ = cs.sessions.SaveAll()
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSessions, Sessions: cs.sessions.List(), SessionID: cs.sessions.CurrentID(),
				})

			case websocket.TypeSessionBranch:
				if _, ok := cs.sessions.Branch(msg.SessionID, msg.Index); !ok {
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtError, Error: "无法创建分支"})
					break
				}
				_ = cs.sessions.SaveAll()
				c.SendEvent(websocket.ServerEvent{
					Type: websocket.EvtSessions, Sessions: cs.sessions.List(), SessionID: cs.sessions.CurrentID(),
				})

			case websocket.TypeAskReply:
				cs.mu.Lock()
				ch, ok := cs.pending[msg.AskID]
				if ok {
					delete(cs.pending, msg.AskID)
				}
				tool := cs.askTool[msg.AskID]
				delete(cs.askTool, msg.AskID)
				cs.mu.Unlock()
				if msg.AskAlways && tool != "" {
					// 当前对话始终允许该工具
					cs.sessions.Current().SetAlwaysAllowed(tool)
				}
				if ok {
					ch <- msg.AskApprove
				}

			case websocket.TypeMessage:
				sessID := cs.sessions.CurrentID()
				if msg.Content == "/clear" {
					cs.sessions.Current().Clear()
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDone, SessionID: sessID})
					return
				}
				// 运行模式切换（纯后端、无需前端改动）：/plan 只读规划，/build 恢复全工具。
				if cmd := strings.TrimSpace(msg.Content); cmd == "/plan" || cmd == "/build" {
					mode := "build"
					if cmd == "/plan" {
						mode = "plan"
					}
					cs.sessions.Current().SetMode(mode)
					_ = cs.sessions.SaveAll()
					note := "已切换到 build 模式（可修改工作区）"
					if mode == "plan" {
						note = "已切换到 plan 模式（只读：仅可读取/搜索，不能修改文件）"
					}
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDelta, Content: note, SessionID: sessID})
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDone, SessionID: sessID})
					return
				}
				// busy 按会话隔离：A 对话处理中时，B 对话仍可并发跑 Agent。
				cs.mu.Lock()
				if cs.busy[sessID] {
					cs.mu.Unlock()
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtError, Error: "该对话上一条消息仍在处理中，请稍候", SessionID: sessID})
					return
				}
				cs.busy[sessID] = true
				msgCtx, msgCancel := context.WithCancel(ctx)
				cs.interruptCancel[sessID] = msgCancel
				cs.mu.Unlock()
				atts := make([]ai.Attachment, 0, len(msg.Attachments))
				for _, a := range msg.Attachments {
					atts = append(atts, ai.Attachment{Type: a.Type, MIMEType: a.MIMEType, Data: a.Data, Filename: a.Filename})
				}
				// Agent 在独立 goroutine 中运行：消息处理是串行的，若在这里同步执行，
				// 停止/工具确认（interrupt/ask_reply）会排在队列里永远处理不到。
				// sessID 在入队时固定，运行中切换 Current 不影响本次归属的会话。
				go func() {
					defer func() {
						cs.mu.Lock()
						delete(cs.busy, sessID)
						delete(cs.interruptCancel, sessID)
						cs.mu.Unlock()
						msgCancel()
					}()
					runServerAgentWithAttachments(msgCtx, st, cs, c, sessID, msg.Content, msg.System, atts)
					_ = cs.sessions.SaveAll()
				}()

			case websocket.TypeInterrupt:
				// 只中断当前正在查看的会话；其它会话的并发运行不受影响。
				sessID := cs.sessions.CurrentID()
				cs.mu.Lock()
				cancel := cs.interruptCancel[sessID]
				cs.mu.Unlock()
				if cancel != nil {
					cancel()
					c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDone, SessionID: sessID})
				}
			}
		})
	})

	authUser, authPass, authEnabled := ResolveAuth(opts.Username, opts.Password)
	auth := newAuthState(authUser, authPass, authEnabled)
	wsState := newWorkspace()
	agent.SetWorkspaceRoot(wsState.Root())

	// Nuxt 静态资源全部 go:embed 打进二进制，由「/」与「/_nuxt/」统一提供
	mux := http.NewServeMux()
	mux.Handle("/login", http.HandlerFunc(auth.handleLogin))
	// 健康检查 / 就绪探针（供容器编排 / 负载均衡，不要求登录）
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ready", handleReady(st))

	// 文件浏览/编辑与工作目录 API
	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleFiles(w, r, wsState)
	})
	mux.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		if r.Method == http.MethodGet {
			handleFile(w, r, wsState)
		} else {
			handleSaveFile(w, r, wsState)
		}
	})
	mux.HandleFunc("/api/mkdir", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleMkdir(w, r, wsState)
	})
	mux.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleExport(w, r)
	})
	mux.HandleFunc("/api/session/export", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleSessionExport(w, r)
	})
	mux.HandleFunc("/api/ca", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleCACerts(w, r)
	})
	mux.HandleFunc("/api/ca/upload", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleCAUpload(w, r)
	})
	mux.HandleFunc("/api/ca/delete", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleCADelete(w, r)
	})
	mux.HandleFunc("/api/shells", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleShells(w, r)
	})
	mux.HandleFunc("/api/import", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleImport(w, r)
	})
	mux.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleDeleteFile(w, r, wsState)
	})
	mux.HandleFunc("/api/chmod", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleChmod(w, r, wsState)
	})
	mux.HandleFunc("/api/chown", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleChown(w, r, wsState)
	})
	mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleUpload(w, r, wsState)
	})
	mux.HandleFunc("/api/download", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleDownload(w, r, wsState)
	})
	mux.HandleFunc("/api/workspace", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		handleWorkspace(w, r, wsState)
	})
	// 工具管理（独立「工具」页面）：列表 / 权限设置 / 删除文件型工具与 MCP 配置
	mux.HandleFunc("/api/tools", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "仅支持 GET"})
			return
		}
		st.handleToolsList(w, r)
	})
	mux.HandleFunc("/api/tools/rule", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "仅支持 POST"})
			return
		}
		st.handleToolsSetRule(w, r)
	})
	mux.HandleFunc("/api/tools/delete", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "仅支持 POST"})
			return
		}
		st.handleToolsDelete(w, r)
	})
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"version": version.Current(),
			"counter": version.Parse(version.Current()),
		})
	})
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		st.mu.RLock()
		cfg := st.settings.AIConfig()
		snap := st.settings.Snapshot()
		st.mu.RUnlock()
		q := r.URL.Query()
		if t := q.Get("type"); t != "" {
			cfg.Type = t
		}
		if b := q.Get("base"); b != "" {
			cfg.BaseURL = b
		}
		if p := q.Get("provider"); p != "" {
			cfg.Provider = p
		}
		// POST：直接带目标厂商的地址/密钥取列表，无需临时切换激活厂商
		//（旧的切换方式会触发 settings 回传，把设置页正在编辑的内容冲掉）。
		if r.Method == http.MethodPost {
			var req struct {
				Type     string `json:"type"`
				BaseURL  string `json:"base_url"`
				APIKey   string `json:"api_key"`
				Provider string `json:"provider"`
				Model    string `json:"model"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
				return
			}
			if req.Type != "" {
				cfg.Type = req.Type
			}
			if req.BaseURL != "" {
				cfg.BaseURL = req.BaseURL
			}
			if req.Provider != "" {
				cfg.Provider = req.Provider
			}
			if req.Model != "" {
				cfg.Model = req.Model
			}
			if req.APIKey != "" && req.APIKey != settings.MaskedAPIKey {
				cfg.APIKey = req.APIKey
			} else {
				// 前端拿到的是掩码密钥：按厂商名回查真实密钥，查不到再用激活厂商的
				key := ""
				for _, p := range snap.Providers {
					if p.Provider == req.Provider {
						key = p.APIKey
						break
					}
				}
				if key == "" {
					key = snap.APIKey
				}
				cfg.APIKey = key
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		models, err := ai.ListModels(ctx, cfg)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider": cfg.Provider,
			"type":     cfg.Type,
			"models":   models,
		})
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, r *http.Request) {
		// 登录信息不需要认证即可查询（仅暴露是否启用，不泄露用户名）。
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":          auth.enabled,
			"default_username": DefaultUsername,
		})
	})
	mux.Handle("/_nuxt/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Nuxt 静态产物资源（公开）：登录页（SPA）也需要加载，故不要求认证。
		nuxt := web.NuxtFS()
		p := strings.TrimPrefix(r.URL.Path, "/")
		if f, err := nuxt.Open(p); err == nil {
			f.Close()
			serveNuxtFile(w, r, nuxt, p)
			return
		}
		http.NotFound(w, r)
	}))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		// Nuxt SPA：先尝试直接命中的静态文件，否则回退 index.html 由前端路由接管（如 /login）。
		// index.html 禁止浏览器缓存，保证升级/改版后刷新即可看到最新版本。
		nuxt := web.NuxtFS()
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := nuxt.Open(p); err == nil {
			f.Close()
			serveNuxtFile(w, r, nuxt, p)
			return
		}
		// 子路由（/settings、/tools）优先命中预渲染的 <path>/index.html
		if !strings.HasSuffix(p, ".html") {
			nested := strings.TrimSuffix(p, "/") + "/index.html"
			if f, err := nuxt.Open(nested); err == nil {
				f.Close()
				serveNuxtFile(w, r, nuxt, nested)
				return
			}
		}
		// 带扩展名的资源路径缺失时必须 404，绝不能回退成 index.html：
		// 否则浏览器会把 HTML 当 JS/CSS 解析，出现前后端产物不一致的诡异问题。
		if path.Ext(p) != "" {
			http.NotFound(w, r)
			return
		}
		serveNuxtFile(w, r, nuxt, "index.html")
	}))
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if !auth.require(w, r) {
			return
		}
		hub.ServeWS(w, r)
	})

	// 安全中间件：跨站请求同源校验 + 基础安全响应头。
	host := listenAddr(opts)
	useTLS := opts.HTTPS || (opts.TLSCert != "" && opts.TLSKey != "")
	var handler http.Handler = mux
	handler = sameOriginGuard(handler)
	handler = securityHeaders(handler, useTLS)

	srv := &http.Server{
		Addr:              listenAddr(opts),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	log.Printf("licode serve 已启动: %s://%s/", scheme, host)
	if strings.HasPrefix(host, "0.0.0.0:") || strings.HasPrefix(host, "0.0.0.0") {
		log.Printf("可用链接: %s://%s:%d/  （局域网/手机访问）", scheme, opts.Host, opts.Port)
	}
	if authEnabled {
		log.Printf("登录已启用（浏览器打开后需登录）")
	} else {
		log.Printf("登录未启用（可用 --password 或环境变量 %s 开启）", EnvPassword)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	// SIGHUP：热重载配置（重读 ~/.licode/config.json 并重建客户端），不中断服务。
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			if err := reloadServerSettings(st); err != nil {
				log.Printf("SIGHUP 重载失败: %v", err)
			} else {
				log.Printf("SIGHUP 已重载配置")
				// RAGSource 可能变化，重建索引
				st.mu.Lock()
				st.rag = nil
				st.mu.Unlock()
			}
		}
	}()
	go func() {
		<-stop
		st.mu.Lock()
		st.shuttingDown = true
		to := st.settings.Snapshot().ShutdownTimeout
		st.mu.Unlock()
		if to <= 0 {
			to = 30
		}
		log.Printf("收到关停信号，优雅退出（等待上限 %ds）…", to)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(to)*time.Second)
		defer cancel()
		// 等待当前 HTTP/WebSocket 请求（含正在运行的 DAG 子代理与 Shell 脚本）自然完成或超时
		_ = srv.Shutdown(ctx)
		// 关闭 MCP 子进程，避免资源泄漏/数据损坏
		agent.CloseMCPClients()
		log.Printf("已停止")
	}()

	if useTLS {
		cert, key := opts.TLSCert, opts.TLSKey
		if cert == "" || key == "" {
			var err error
			cert, key, err = ensureSelfSignedCert()
			if err != nil {
				return fmt.Errorf("自动生成证书失败: %w", err)
			}
			log.Printf("已自动生成自签名证书：%s", cert)
		}
		err = srv.ListenAndServeTLS(cert, key)
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
	err = srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// applyServerSettings 校验并应用新的设置，重建客户端。
func applyServerSettings(st *serverState, msg websocket.ClientMessage) error {
	// msg.Settings 已是反序列化后的结构，直接转换，无需再 Marshal/Unmarshal 一轮。
	data, err := json.Marshal(msg.Settings)
	if err != nil {
		return fmt.Errorf("设置格式错误: %w", err)
	}
	var s settings.Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("设置格式错误: %w", err)
	}
	// 前端回传的密钥若仍是掩码占位符，用现有真实密钥还原（密钥不出服务器）。
	st.mu.RLock()
	s.RestoreMaskedKeys(st.settings)
	st.mu.RUnlock()
	s.EnsureDefaults()
	if err := s.Validate(); err != nil {
		return fmt.Errorf("设置无效: %v", err)
	}
	client, err := s.NewClient()
	if err != nil {
		return err
	}
	st.mu.Lock()
	st.settings = s.Snapshot()
	st.client = client
	st.mu.Unlock()
	return nil
}

// reloadServerSettings 从磁盘重读配置并重建客户端（热重载，SIGHUP 触发）。
// 与启动路径一致：AI/API 设置来自 ~/.licode/config.json（settings.Load），
// 而非 config.toml（后者仅含服务器监听项），避免热重载读到不同来源。
func reloadServerSettings(st *serverState) error {
	s, err := settings.Load()
	if err != nil {
		return err
	}
	client, err := s.NewClient()
	if err != nil {
		return err
	}
	st.mu.Lock()
	st.settings = s.Snapshot()
	st.client = client
	st.mu.Unlock()
	return nil
}

// runServerAgentWithAttachments 在指定会话下运行一次 Agent，支持附件。
// sessID 在消息入队时固定，与连接的 Current 解耦，避免并发会话串台。
func runServerAgentWithAttachments(ctx context.Context, st *serverState, cs *connState, c *websocket.Client, sessID, content, roleSystem string, attachments []ai.Attachment) {
	st.mu.RLock()
	s := st.settings.Snapshot()
	client := st.client
	st.mu.RUnlock()

	sess, ok := cs.sessions.Get(sessID)
	if !ok {
		c.SendEvent(websocket.ServerEvent{Type: websocket.EvtError, Error: "会话不存在", SessionID: sessID})
		return
	}
	if s.TitleGen && sess.Title() == "新对话" {
		sess.SetTitle(autoTitle(content))
	}

	ag := s.BuildAgent(client)
	if roleSystem != "" {
		ag.System = roleSystem + "\n" + ag.System
	}
	ag.Session = sess
	// 运行模式来自会话（/plan 只读、/build 全工具），默认 build。
	ag.Mode = sess.Mode()
	// MaxCtxTokens 必须落在真实会话上（BuildAgent 里的临时会话已被上面的
	// 赋值覆盖，若不重设则上下文窗口保护会静默失效）。
	if s.MaxCtxTokens > 0 {
		sess.SetMaxTokens(s.MaxCtxTokens)
	}
		defer ag.Close()
		ag.Ask = func(ctx context.Context, toolName, args string) (bool, error) {
			if s.AutoAllow || sess.AlwaysAllowed(toolName) {
				return true, nil
			}
			askID := fmt.Sprintf("ask-%d", cs.askSeq.Add(1))
			ch := make(chan bool, 1)
			cs.mu.Lock()
			cs.pending[askID] = ch
			cs.askTool[askID] = toolName
			cs.mu.Unlock()
			// 无论因何退出（收到回复/上下文取消/Agent 异常结束），都清理映射，
			// 避免前端不回复时 pending/askTool 条目永久驻留泄漏。
			defer func() {
				cs.mu.Lock()
				delete(cs.pending, askID)
				delete(cs.askTool, askID)
				cs.mu.Unlock()
			}()
			c.SendEvent(websocket.ServerEvent{
				Type: websocket.EvtAsk, ToolName: toolName, ToolArgs: args, AskID: askID, SessionID: sessID,
			})
			select {
			case ok := <-ch:
				return ok, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}

	stream := true
	if s.Streaming != nil {
		stream = *s.Streaming
	}
	if s.RAGEnabled {
		if snippets := st.ragLookup(content, s.RAGSource, s.RAGTopFiles); snippets != "" {
			// RAG 片段属易变上下文，注入 C 区尾部（并入最后一条 user 消息），
			// 不追加到 ag.System，以免破坏冻结前缀、抬高前缀缓存失效成本。
			ag.ContextExtra = "以下是用户当前项目中的相关源码片段（来自 RAG 检索），" +
				"请优先据此准确回答，不要编造不存在的内容：\n" + snippets
		}
	}
	var textBuf strings.Builder
	_ = ag.RunWithAttachments(ctx, content, attachments, func(e agent.Event) error {
		if e.Type == agent.EventText && !stream {
			textBuf.WriteString(e.Content)
		} else if e.Type == agent.EventText {
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDelta, Content: e.Content, SessionID: sessID})
		} else if e.Type == agent.EventToolStart {
			c.SendEvent(websocket.ServerEvent{
				Type: websocket.EvtToolStart, ToolName: e.ToolName, ToolArgs: e.ToolArgs, SessionID: sessID,
			})
		} else if e.Type == agent.EventToolDone {
			c.SendEvent(websocket.ServerEvent{
				Type: websocket.EvtToolDone, ToolName: e.ToolName, ToolOut: e.ToolOut, SessionID: sessID,
			})
		} else if e.Type == agent.EventDone {
			if !stream && textBuf.Len() > 0 {
				c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDelta, Content: textBuf.String(), SessionID: sessID})
			}
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtDone, SessionID: sessID})
		} else if e.Type == agent.EventError {
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtError, Error: e.Error, SessionID: sessID})
		} else if e.Type == agent.EventReasoning {
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtReasoning, Content: e.Content, SessionID: sessID})
		} else if e.Type == agent.EventStatus {
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtStatus, Content: e.Content, SessionID: sessID})
		} else if e.Type == agent.EventStats {
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtStats, Stats: e.Stats, SessionID: sessID})
		}
		return nil
	})
	// 把本次运行的 token 用量（含缓存命中）累计到会话并落盘，
	// 之前 ag.Usage 只在内存里累加、从不回写，导致会话/前端看不到用量。
	sess.AddUsage(ag.Usage)
}

// autoTitle 从第一条用户消息生成对话标题。
func autoTitle(content string) string {
	trimmed := strings.TrimSpace(content)
	if utf8.RuneCountInString(trimmed) > 18 {
		runes := []rune(trimmed)
		return string(runes[:18])
	}
	return trimmed
}

// serveNuxtFile 以正确的 Content-Type 与缓存策略提供 Nuxt 静态产物文件。
// .html 不缓存（改版即时生效）；/_nuxt/ 资源名带内容哈希，可长缓存。
// 目录路径（/settings、/tools）会解析到其 index.html，避免被当成文件服务而 301。
func serveNuxtFile(w http.ResponseWriter, r *http.Request, nuxt fs.FS, name string) {
	// 安全校验：防止路径遍历攻击
	cleanName := filepath.Clean(name)
	if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
		http.NotFound(w, r)
		return
	}
	
	info, err := fs.Stat(nuxt, cleanName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		idx := strings.TrimSuffix(cleanName, "/") + "/index.html"
		fi, ierr := fs.Stat(nuxt, idx)
		if ierr != nil || fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		cleanName = idx
	}
	if strings.HasSuffix(cleanName, ".html") {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=604800")
	}
	http.ServeFile(w, r, filepath.Join("nuxtweb", cleanName))
}

// isUnsafeMethod 判断是否为写请求方法（GET/HEAD/OPTIONS 之外）。
func isUnsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// sameOriginGuard 对写请求做同源校验：若请求带 Origin 头且与 Host 不符则拒绝，
// 防止跨站伪造请求访问带 Cookie 的 /api/* 写接口与 /ws 升级。无 Origin（curl/脚本）或同源放行。
func sameOriginGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (isUnsafeMethod(r.Method) && strings.HasPrefix(r.URL.Path, "/api/")) || r.URL.Path == "/ws" {
			if o := r.Header.Get("Origin"); o != "" && !originMatchesHost(o, r.Host) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("403 跨站请求已被拒绝"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func originMatchesHost(origin, host string) bool {
	ou, err := url.Parse(origin)
	if err != nil || ou.Host == "" {
		return false
	}
	if strings.EqualFold(ou.Host, host) {
		return true
	}
	// 兼容 Host 带/不带默认端口的差异。
	h := host
	if hh, _, e := net.SplitHostPort(host); e == nil {
		h = hh
	}
	return strings.EqualFold(ou.Hostname(), h)
}

// securityHeaders 追加基础安全响应头。
func securityHeaders(next http.Handler, tls bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// CSP：禁止对象嵌入与外部脚本，允许内联样式（Nuxt 产物需要）；
		// connect-src 放开 ws/wss 以支持同源 WebSocket。
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' ws: wss:; "+
				"object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		if tls {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
