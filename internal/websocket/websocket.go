// Package websocket implements the connection hub used by the web UI.
// Each connected client owns an agent + session on the server; user messages
// trigger agent runs whose events are streamed back over the socket.
package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Message types (client -> server).
const (
	TypeMessage        = "message" // {content}
	TypePing           = "ping"
	TypeSettingsGet    = "settings_get"
	TypeSettingsSet    = "settings_set"
	TypeAskReply       = "ask_reply"
	TypeInterrupt      = "interrupt"
	TypeSessionsGet    = "sessions_get"
	TypeSessionNew     = "session_new"
	TypeSessionSwitch  = "session_switch"  // {session_id}
	TypeSessionRename  = "session_rename"  // {session_id, content}
	TypeSessionDelete  = "session_delete"  // {session_id}
	TypeSessionBranch  = "session_branch"  // {session_id, index, content}
	TypeSessionHistory = "session_history" // {session_id} 请求某会话的完整历史消息
)

// Event types (server -> client), mirroring agent.Event.
const (
	EvtDelta     = "delta"
	EvtToolStart = "tool_start"
	EvtToolDone  = "tool_done"
	EvtDone      = "done"
	EvtError     = "error"
	EvtStatus    = "status"
	EvtReasoning = "reasoning"
	// EvtPluginOutput 插件斜杠命令的输出（content 为文本）。
	EvtPluginOutput = "plugin_output"
	EvtSettings  = "settings"
	EvtAsk       = "ask"
	EvtSessions  = "sessions"
	EvtStats     = "stats"
	// EvtHistory 回放某个会话的完整历史消息（存放于 ~/.licode/sessions/*.json）。
	EvtHistory = "history"
)

// Broadcast 向所有已连接客户端发送事件。
func (h *Hub) Broadcast(ev ServerEvent) {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()
	for _, c := range clients {
		c.SendEvent(ev)
	}
}

// ServerEvent is a JSON event streamed to clients.
type ServerEvent struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	ToolName  string `json:"toolName,omitempty"`
	ToolArgs  string `json:"toolArgs,omitempty"`
	ToolOut   string `json:"toolOut,omitempty"`
	Error     string `json:"error,omitempty"`
	Settings  any    `json:"settings,omitempty"`
	Sessions  any    `json:"sessions,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Stats     any    `json:"stats,omitempty"`
	// Messages 携带会话历史（EvtHistory）。
	Messages any `json:"messages,omitempty"`
	// AskID 标识一次待确认的工具调用。
	AskID string `json:"askId,omitempty"`
}

// ClientMessage is a request sent from a client.
type ClientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	// System 可选：来自当前"角色"的系统提示词覆盖。非空时前置到默认系统提示词。
	System   string `json:"system,omitempty"`
	Settings any    `json:"settings,omitempty"`
	// AskReply 对应 AskID 的确认结果。
	AskID      string `json:"askId,omitempty"`
	AskApprove bool   `json:"askApprove,omitempty"`
	AskAlways  bool   `json:"askAlways,omitempty"`
	SessionID    string        `json:"sessionId,omitempty"`
	Index        int           `json:"index,omitempty"` // {session_branch} 分支点消息序号
	Attachments  []Attachment  `json:"attachments,omitempty"`
}

// Attachment 是多模态附件（图片/文件），base64 编码。
type Attachment struct {
	Type     string `json:"type"`      // "image" 或 "file"
	MIMEType string `json:"mime_type"` // 如 "image/png"
	Data     string `json:"data"`      // base64 编码
	Filename string `json:"filename,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// 无 Origin（curl/脚本/移动 App）放行；浏览器请求必须同源，
		// 防止任意网页通过 ws:// 直连本地服务（未启用登录时等同匿名 RCE）。
		// 经反代/局域网 IP 访问时浏览器 Origin 与 Host 一致，不会误拒。
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return originMatchesHost(origin, r.Host)
	},
}

// originMatchesHost 校验 Origin 与请求 Host 是否同源（兼容默认端口的差异）。
func originMatchesHost(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, host) {
		return true
	}
	h := host
	if hh, _, e := net.SplitHostPort(host); e == nil {
		h = hh
	}
	return strings.EqualFold(u.Hostname(), h)
}

// Handler is called with each client that connects. The Hub does not know
// about agents; the server wires them in via this callback.
type Handler func(ctx context.Context, c *Client)

// Hub tracks connected clients and dispatches connections to a Handler.
type Hub struct {
	mu      sync.Mutex
	clients map[*Client]struct{}
	onConn  Handler
}

func NewHub() *Hub {
	return &Hub{clients: map[*Client]struct{}{}}
}

// OnConnect registers the per-connection handler.
func (h *Hub) OnConnect(fn Handler) {
	h.onConn = fn
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	// 注意：不 close(c.send)。断开后 Agent 端的回调可能仍在写该 channel，
	// close 会触发 send-on-closed-channel panic；writePump 由 ctx 取消退出。
}

func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// ServeWS upgrades an HTTP request and runs the client lifecycle.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	c := NewClient(h, conn)
	h.register(c)

	ctx, cancel := context.WithCancel(r.Context())
	// 同步注册消息处理器：若放 goroutine 里，客户端在连接瞬间发来的
	// 第一条消息（settings_get/sessions_get）会在注册前被 processMessages 丢弃。
	if h.onConn != nil {
		h.onConn(ctx, c)
	}
	go c.writePump(ctx)
	go c.processMessages(ctx)

	c.readPump(ctx)

	cancel()
	c.cancel()
	h.unregister(c)
}

// Client represents one WebSocket connection. It has its own send queue so
// slow clients never block the agent goroutine.
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	mu            sync.Mutex
	onUserMessage func(ctx context.Context, msg ClientMessage)
	msgQueue      chan ClientMessage
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		msgQueue: make(chan ClientMessage, 50),
		ctx:      ctx,
		cancel:   cancel,
	}
	return c
}

// SendEvent marshals and queues an event for delivery.
func (c *Client) SendEvent(evt ServerEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		// Client is backed up; drop rather than block the agent.
	}
}

// SendRaw queues pre-marshaled bytes.
func (c *Client) SendRaw(data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.mu.Lock()
			err := c.conn.WriteMessage(websocket.TextMessage, data)
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(ctx context.Context) {
	defer c.conn.Close()
	defer c.cancel()
	// 读循环退出后不会再有消息入队；关闭队列让 processMessages 随之退出，
	// 否则每条连接泄漏一个常驻 goroutine（连带整个 connState 无法回收）。
	defer close(c.msgQueue)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		select {
		case c.msgQueue <- msg:
		default:
			c.SendEvent(ServerEvent{
				Type:  EvtError,
				Error: "消息队列已满，请稍候",
			})
		}
	}
}

func (c *Client) processMessages(ctx context.Context) {
	for msg := range c.msgQueue {
		if c.onUserMessage != nil {
			c.onUserMessage(ctx, msg)
		}
	}
}

// OnUserMessage lets the server attach a handler for every client message.
func (c *Client) OnUserMessage(fn func(ctx context.Context, msg ClientMessage)) {
	c.onUserMessage = fn
}
