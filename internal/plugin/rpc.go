package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"licode/internal/procutil"
)

// rpcRequest 是发给插件的 JSON-RPC 请求。
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse 是插件返回的 JSON-RPC 响应。
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"` // 插件主动通知（log/status）
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// process 是一个插件子进程及其 JSON-RPC 连接。
type process struct {
	cmd     *exec.Cmd
	stdin   *bufio.Writer
	stdout  *bufio.Reader
	mu      sync.Mutex
	pending map[int]chan *rpcResponse
	nextID  int
	closeCh chan struct{}
	once    sync.Once
	// notify 回调：处理插件主动发来的通知（log/status）。
	notify func(method string, params json.RawMessage)
}

// startProcess 启动插件进程。env 为额外环境变量；inheritEnv 决定是否继承宿主环境。
func startProcess(dir, entry string, args []string, env map[string]string, inheritEnv bool) (*process, error) {
	bin, err := procutil.ResolveEntry(entry)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	base := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "LANG=" + os.Getenv("LANG")}
	if inheritEnv {
		base = os.Environ()
	}
	for k, v := range env {
		base = append(base, k+"="+v)
	}
	cmd.Env = base

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动插件进程失败: %w", err)
	}
	p := &process{
		cmd:     cmd,
		stdin:   bufio.NewWriter(stdin),
		stdout:  bufio.NewReader(stdout),
		pending: map[int]chan *rpcResponse{},
		closeCh: make(chan struct{}),
	}
	go p.readLoop()
	return p, nil
}

func (p *process) readLoop() {
	for {
		select {
		case <-p.closeCh:
			return
		default:
		}
		payload, err := readFrame(p.stdout)
		if err != nil {
			return
		}
		var msg rpcResponse
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		if msg.ID == nil {
			// 主动通知（log/status）
			if msg.Method != "" && p.notify != nil {
				p.notify(msg.Method, msg.Params)
			}
			continue
		}
		p.mu.Lock()
		ch := p.pending[*msg.ID]
		delete(p.pending, *msg.ID)
		p.mu.Unlock()
		if ch != nil {
			select {
			case ch <- &msg:
			default:
			}
		}
	}
}

// readFrame 同时支持 Content-Length 帧与 NDJSON 行（与 MCP 一致）。
func readFrame(r *bufio.Reader) ([]byte, error) {
	peek, err := r.Peek(1)
	if err != nil {
		return nil, err
	}
	if peek[0] != 'C' {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return bytes.TrimSpace(line), nil
	}
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if i := strings.Index(line, ":"); i >= 0 {
			if strings.EqualFold(strings.TrimSpace(line[:i]), "Content-Length") {
				if n, aerr := strconv.Atoi(strings.TrimSpace(line[i+1:])); aerr == nil {
					contentLength = n
				}
			}
		}
	}
	if contentLength < 0 || contentLength > 8<<20 {
		return nil, fmt.Errorf("无效 Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(body), nil
}

// call 发送请求并等待响应（受 ctx 超时约束）。
func (p *process) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	p.mu.Lock()
	p.nextID++
	id := p.nextID
	ch := make(chan *rpcResponse, 1)
	p.pending[id] = ch
	p.mu.Unlock()

	if err := p.write(id, method, params); err != nil {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, err
	}
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("%s", resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, ctx.Err()
	case <-p.closeCh:
		return nil, fmt.Errorf("插件进程已退出")
	}
}

// notify 发送通知（无需响应）。
func (p *process) notifyHost(method string, params any) error {
	return p.write(0, method, params)
}

func (p *process) write(id int, method string, params any) error {
	req := rpcRequest{JSONRPC: "2.0", Method: method}
	if id > 0 {
		req.ID = &id
	}
	if params != nil {
		req.Params = params
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := fmt.Fprintf(p.stdin, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	if _, err := p.stdin.Write(b); err != nil {
		return err
	}
	return p.stdin.Flush()
}

func (p *process) close() {
	p.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = p.call(ctx, "shutdown", nil)
		close(p.closeCh)
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
			_, _ = p.cmd.Process.Wait()
		}
	})
}
