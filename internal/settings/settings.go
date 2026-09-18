// Package settings 提供运行时可变的应用设置。设置可在 Web 界面中实时修改，
// 并立即生效。
package settings

import (
	"errors"
	"net/url"
	"runtime"
	"strings"
	"time"

	"licode/internal/agent"
	"licode/internal/ai"
	"licode/internal/dnsclient"
)

var (
	errToolNameRequired = errors.New("工具名不能为空")
	errToolRuleInvalid  = errors.New("规则取值无效（应为 允许/询问/禁止）")
)

// ProviderChoices 是可选厂商。
var ProviderChoices = []string{"openai", "claude", "ollama", "gemini"}

// ProviderConfig 描述一个已配置的厂商条目（可自定义名称与协议类型）。
type ProviderConfig struct {
	Provider string   `json:"provider"` // 标识（内置名或自定义）
	Name     string   `json:"name"`     // 自定义显示名称
	Type     string   `json:"type"`     // 协议类型：openai/claude/ollama/gemini；空按 Provider 推断
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	Model    string   `json:"model"`            // 当前使用的模型
	Models   []string `json:"models,omitempty"` // 该厂商的模型列表（可自由增删，仅作展示/选择用）
	HostIP      string `json:"host_ip,omitempty"`    // 指定 IP：该厂商 base_url 域名直接连此 IP（SNI/证书校验仍用原域名），绕过 DNS 劫持
	InsecureSSL bool   `json:"insecure_ssl,omitempty"` // 忽略 TLS 证书校验（仅限自签名证书等受控场景）
}

// AddModel 把模型追加进列表（去重），供激活与导入时保持一致性。
func (p *ProviderConfig) AddModel(m string) {
	if m == "" {
		return
	}
	for _, x := range p.Models {
		if x == m {
			return
		}
	}
	p.Models = append(p.Models, m)
}

// resolveType 推断协议类型（openai / claude / google；ollama 归 openai）。
func (p *ProviderConfig) resolveType() string {
	t := strings.ToLower(p.Type)
	switch t {
	case "openai", "claude", "google":
		return t
	case "ollama":
		return "openai"
	case "gemini", "anthropic":
		if t == "gemini" {
			return "google"
		}
		return "claude"
	}
	switch strings.ToLower(p.Provider) {
	case "openai", "ollama", "custom":
		return "openai"
	case "claude", "anthropic":
		return "claude"
	case "gemini", "google":
		return "google"
	}
	return "openai" // 自定义厂商默认 OpenAI 兼容
}

// DisplayName 显示名称：Name > Provider。
func (p *ProviderConfig) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Provider
}

// MCPServer 描述一个 MCP 服务器（stdio 进程）。
type MCPServer = agent.MCPServer

// Settings 是运行时可变的全部应用设置。
type Settings struct {
	Provider      string            `json:"provider"`
	BaseURL       string            `json:"base_url"`
	APIKey        string            `json:"api_key"`
	Model         string            `json:"model"`
	Providers     []ProviderConfig  `json:"providers"` // 已配置的多个厂商
	Temperature   float64           `json:"temperature"`
	MaxTokens     int               `json:"max_tokens"`
	MaxIterations int               `json:"max_iterations"`
	SubAgents     bool              `json:"subagents"`
	AskTools      []string          `json:"ask_tools"`
	DenyTools     []string          `json:"deny_tools"`
	Compaction    bool              `json:"compaction"`     // 上下文超限时用 LLM 压缩
	TitleGen      bool              `json:"title_gen"`      // 自动生成对话标题
	AutoAllow     bool              `json:"auto_allow"`     // 风险工具自动允许
	Streaming     *bool             `json:"streaming"`      // 流式输出（nil=默认开）
	ToolRules     map[string]string `json:"tool_rules"`     // 工具名 -> allow/ask/deny
	MCPServers    []MCPServer       `json:"mcp_servers"`    // MCP 服务器列表
	ShellPath     string            `json:"shell_path"`     // Shell 路径（默认 /bin/sh）
	RetryMax      int               `json:"retry_max"`      // LLM 调用失败重试次数（0=关闭）
	SubTimeout    int               `json:"sub_timeout"`    // 子代理硬超时（秒，0=不限制）
	MaxCtxTokens  int               `json:"max_ctx_tokens"` // 上下文窗口保护阈值（0=关闭）
	RedactSecrets bool              `json:"redact_secrets"` // 工具输出敏感信息脱敏
	// 特性1：语义缓存
	CacheEnabled bool `json:"cache_enabled"` // 开启问题-结果缓存
	CacheTTL     int  `json:"cache_ttl"`     // 缓存有效期（秒，默认 3600）
	// 特性3：迭代式工具调用
	ToolAutoRetry bool `json:"tool_auto_retry"` // 空结果/错误时自动重试
	ToolRetryMax  int  `json:"tool_retry_max"`  // 最多重试次数（默认 3）
	// 特性4：优雅退出等待时间
	ShutdownTimeout int `json:"shutdown_timeout"` // 关停等待当前任务完成的秒数（默认 30）
	// 特性5：轻量 RAG
	RAGEnabled  bool   `json:"rag_enabled"`   // 索引项目源码并注入上下文
	RAGSource   string `json:"rag_source"`    // 索引根目录（默认当前工作目录）
	RAGTopFiles int    `json:"rag_top_files"` // 注入的最大文件数（默认 5）
	// DNS 自定义解析配置，用于解决 API 端点域名解析失败或 DNS 污染导致的 connection refused。
	DNS *dnsclient.Config `json:"dns,omitempty"`
}

// Presets 是常见 DNS 厂商的预设。

// Defaults 返回合并了配置文件、环境变量与内置默认值的初始设置。
func Defaults() Settings {
	s, _ := Load()
	return s
}

// ApplyFlags 用命令行参数覆盖初始设置。
func (s *Settings) ApplyFlags(noSubAgents bool) {
	if noSubAgents {
		s.SubAgents = false
	}
	s.UpsertActive()
	s.syncTopLevel()
}

// UpsertActive 把当前激活厂商的信息写回 Providers 列表。
func (s *Settings) UpsertActive() {
	for i := range s.Providers {
		if s.Providers[i].Provider == s.Provider {
			if s.BaseURL != "" {
				s.Providers[i].BaseURL = s.BaseURL
			}
			if s.APIKey != "" {
				s.Providers[i].APIKey = s.APIKey
			}
			if s.Model != "" {
				s.Providers[i].Model = s.Model
				s.Providers[i].AddModel(s.Model)
			}
			s.Providers[i].Type = s.Providers[i].resolveType()
			return
		}
	}
	pc := ProviderConfig{
		Provider: s.Provider,
		BaseURL:  s.BaseURL,
		APIKey:   s.APIKey,
		Model:    s.Model,
	}
	if s.Model != "" {
		pc.AddModel(s.Model)
	}
	if d, ok := ai.Defaults[strings.ToLower(s.Provider)]; ok && pc.BaseURL == "" {
		pc.BaseURL = d.BaseURL
	}
	if d, ok := ai.Defaults[strings.ToLower(s.Provider)]; ok && pc.Model == "" {
		pc.Model = d.Model
	}
	pc.Type = pc.resolveType()
	s.Providers = append(s.Providers, pc)
}

// syncTopLevel 把激活厂商条目同步到顶层字段。
func (s *Settings) syncTopLevel() {
	if s.Provider == "" {
		return
	}
	for _, p := range s.Providers {
		if p.Provider == s.Provider {
			s.BaseURL = p.BaseURL
			s.APIKey = p.APIKey
			if s.Model == "" || p.Model != "" {
				s.Model = p.Model
			}
			return
		}
	}
}

// ActiveProvider 返回当前激活的厂商配置。
func (s *Settings) ActiveProvider() ProviderConfig {
	for _, p := range s.Providers {
		if p.Provider == s.Provider {
			if p.Model == "" {
				p.Model = s.Model
			}
			return p
		}
	}
	return ProviderConfig{Provider: s.Provider, BaseURL: s.BaseURL, APIKey: s.APIKey, Model: s.Model}
}

// SetActiveProvider 切换到指定厂商（未配置则创建默认条目）。
func (s *Settings) SetActiveProvider(name string) {
	s.Provider = name
	s.UpsertActive()
	s.syncTopLevel()
}

// AIConfig 把激活厂商设置转成 ai.Config。
func (s *Settings) AIConfig() ai.Config {
	pc := s.ActiveProvider()
	cfg := ai.Config{
		Provider:    pc.DisplayName(),
		Type:        pc.resolveType(),
		BaseURL:     pc.BaseURL,
		APIKey:      pc.APIKey,
		Model:       pc.Model,
		RetryMax:    s.RetryMax,
		InsecureSSL: pc.InsecureSSL,
	}
	if ip := strings.TrimSpace(pc.HostIP); ip != "" {
		cfg.HostIP = ip
		// 把厂商域名 → 指定 IP 注入 DNS 静态映射（拷贝一份避免改共享配置）。
		d := dnsclient.Config{Mode: s.DNS.Mode, Servers: s.DNS.Servers, Concurrency: s.DNS.Concurrency, TimeoutMS: s.DNS.TimeoutMS}
		if s.DNS.HostOverrides != nil {
			d.HostOverrides = make(map[string]string, len(s.DNS.HostOverrides)+1)
			for k, v := range s.DNS.HostOverrides {
				d.HostOverrides[k] = v
			}
		}
		if host := hostOf(pc.BaseURL); host != "" {
			if d.HostOverrides == nil {
				d.HostOverrides = map[string]string{}
			}
			d.HostOverrides[strings.ToLower(host)] = ip
		}
		cfg.DNS = &d
	} else if s.DNS != nil {
		cfg.DNS = s.DNS
	}
	return cfg
}

// hostOf 提取 URL 的主机名（解析失败返回空）。
func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// NewClient 根据激活厂商创建设置 LLM 客户端。
func (s *Settings) NewClient() (ai.LLMClient, error) {
	return ai.New(s.AIConfig())
}

// BuildAgent 根据设置构建一个完整的 Agent（含子代理、Skills、MCP、压缩）。
// riskyToolDefaults 是默认需要用户确认（ask）的高风险工具名。
// 联网搜索（WebFetch）已移除，后续恢复该功能时需同步加回。
var riskyToolDefaults = []string{"Shell", "Write", "Edit", "Delete", "Move"}

// safeToolDefaults 只读、无副作用的内置工具，显式 allow（配合 "*"=ask 兜底）。
var safeToolDefaults = []string{"Read", "ListDirectory", "Glob", "Grep"}

func (s *Settings) BuildAgent(client ai.LLMClient) *agent.Agent {
	// 特性1：语义缓存（问题-结果缓存，命中即返回，跳过 LLM 调用）
	if s.CacheEnabled {
		client = ai.CacheDecorator(client, ai.NewCache(CacheDir(), s.CacheTTL))
	}
	// 主提示词注入运行时变量：工作目录、操作系统、日期、模型名/ID。
	// 模型名来自激活厂商配置（切换模型时提示词自动跟随）；provider 名作为可读名。
	pc := s.ActiveProvider()
	modelName := pc.Model
	if modelName == "" {
		modelName = client.Model()
	}
	modelID := pc.Provider + "/" + modelName
	if pc.Provider == "" {
		modelID = modelName
	}
	prompt := agent.BuildMainPrompt(BaseDir(), runtime.GOOS, time.Now().Format("2006-01-02"), modelName, modelID)
	ag := agent.NewAgent(client, prompt)
	// 特性6：把 fsnotify 热加载的外部命令工具并入当前 Agent（动态增/删）。
	ag.Tools.MergeFrom(agent.ExternalTools)
	// 附加提示词：~/.licode/md/ 下的所有 .md（默认空）
	if md, err := ReadMDPrompts(MDPromptDir()); err == nil && md != "" {
		ag.System += "\n" + md
	}
	ag.MaxTokens = s.MaxTokens
	ag.MaxIterations = s.MaxIterations
	ag.Temperature = s.Temperature
	ag.Compaction = s.Compaction
	ag.Timeout = s.SubTimeout
	ag.RedactSecrets = s.RedactSecrets
	ag.ToolAutoRetry = s.ToolAutoRetry
	ag.ToolRetryMax = s.ToolRetryMax
	ag.Shell = agent.ShellConfig{Path: s.ShellPath}
	// 安全默认：未知工具（MCP、外部热加载命令、WASM 插件）一律先 "ask"，
	// 只读工具显式放行，高风险工具强制 "ask"。防止未配置任何工具规则时
	// 模型经提示注入通过 MCP/外部工具自主执行任意操作。
	ag.Permissions["*"] = "ask"
	for _, t := range safeToolDefaults {
		ag.Permissions[t] = "allow"
	}
	for _, t := range riskyToolDefaults {
		ag.Permissions[t] = "ask"
	}
	// 工具规则：tool -> allow/ask/deny（优先级高于安全默认，未配置仍默认为 ask）
	for tool, mode := range s.ToolRules {
		if mode == "deny" || mode == "ask" || mode == "allow" {
			ag.Permissions[tool] = mode
		}
	}
	// 兼容旧的 ask_tools / deny_tools
	for _, t := range s.DenyTools {
		ag.Permissions[t] = "deny"
	}
	for _, t := range s.AskTools {
		ag.Permissions[t] = "ask"
	}
	if s.SubAgents {
		ag.RegisterSubAgents(agent.DefaultSubAgentSpecs(client, ag.Shell, s.SubTimeout))
	}
	agent.RegisterSkills(ag.Tools, agent.LoadSkills(agent.SkillDirs()...))
	// MCP 连接管理器绑定到本 Agent，由调用方在运行结束后 Close，
	// 避免每条消息 spawn 的 stdio 子进程存活到进程退出。
	if mgr, err := agent.RegisterMCPServers(ag.Tools, s.MCPServers); err == nil && mgr != nil {
		ag.SetMCPManager(mgr)
	}
	return ag
}

// Snapshot 返回设置的深拷贝，避免并发读写。
func (s *Settings) Snapshot() Settings {
	out := Settings{
		Provider:        s.Provider,
		BaseURL:         s.BaseURL,
		APIKey:          s.APIKey,
		Model:           s.Model,
		Providers:       copyProviders(s.Providers),
		Temperature:     s.Temperature,
		MaxTokens:       s.MaxTokens,
		MaxIterations:   s.MaxIterations,
		SubAgents:       s.SubAgents,
		AskTools:        append([]string{}, s.AskTools...),
		DenyTools:       append([]string{}, s.DenyTools...),
		Compaction:      s.Compaction,
		TitleGen:        s.TitleGen,
		AutoAllow:       s.AutoAllow,
		Streaming:       s.Streaming,
		MCPServers:      append([]MCPServer{}, s.MCPServers...),
		ToolRules:       map[string]string{},
		ShellPath:       s.ShellPath,
		RetryMax:        s.RetryMax,
		SubTimeout:      s.SubTimeout,
		MaxCtxTokens:    s.MaxCtxTokens,
		RedactSecrets:   s.RedactSecrets,
		CacheEnabled:    s.CacheEnabled,
		CacheTTL:        s.CacheTTL,
		ToolAutoRetry:   s.ToolAutoRetry,
		ToolRetryMax:    s.ToolRetryMax,
		ShutdownTimeout: s.ShutdownTimeout,
		RAGEnabled:      s.RAGEnabled,
		RAGSource:       s.RAGSource,
		RAGTopFiles:     s.RAGTopFiles,
	}
	if s.DNS != nil {
		servers := make([]dnsclient.Server, len(s.DNS.Servers))
		copy(servers, s.DNS.Servers)
		d := &dnsclient.Config{Mode: s.DNS.Mode, Servers: servers, Concurrency: s.DNS.Concurrency, TimeoutMS: s.DNS.TimeoutMS}
		if s.DNS.HostOverrides != nil {
			d.HostOverrides = make(map[string]string, len(s.DNS.HostOverrides))
			for k, v := range s.DNS.HostOverrides {
				d.HostOverrides[k] = v
			}
		}
		out.DNS = d
	}
	for k, v := range s.ToolRules {
		out.ToolRules[k] = v
	}
	return out
}

// Validate 校验设置值是否可用。
func (s *Settings) Validate() error {
	_, err := ai.New(s.AIConfig())
	return err
}

// EnsureDefaults 补全默认值（提供商模型、风险工具规则等）。
func (s *Settings) EnsureDefaults() {
	_ = s.finalize()
}

// copyProviders 深拷贝厂商列表（含其内部的 models 切片），避免快照共享底层数组。
func copyProviders(in []ProviderConfig) []ProviderConfig {
	out := make([]ProviderConfig, len(in))
	for i, p := range in {
		out[i] = p
		if len(p.Models) > 0 {
			out[i].Models = append([]string{}, p.Models...)
		}
	}
	return out
}

// EffectiveToolRule 返回工具在当前设置下的最终权限（allow/ask/deny），
// 与 BuildAgent 的装配逻辑保持一致，供工具管理界面展示。
func (s *Settings) EffectiveToolRule(tool string) string {
	rule := "ask" // 未知工具（MCP/外部命令/技能）默认 ask
	for _, t := range safeToolDefaults {
		if t == tool {
			rule = "allow"
			break
		}
	}
	for _, t := range riskyToolDefaults {
		if t == tool {
			rule = "ask"
			break
		}
	}
	if m, ok := s.ToolRules[tool]; ok && (m == "allow" || m == "ask" || m == "deny") {
		rule = m
	}
	for _, t := range s.DenyTools {
		if t == tool {
			rule = "deny"
		}
	}
	for _, t := range s.AskTools {
		if t == tool {
			rule = "ask"
		}
	}
	return rule
}

// SetToolRule 设置某工具的权限规则，并清理互斥的旧式列表条目。
func (s *Settings) SetToolRule(tool, rule string) error {
	if tool == "" {
		return errToolNameRequired
	}
	switch rule {
	case "allow", "ask", "deny":
	default:
		return errToolRuleInvalid
	}
	if s.ToolRules == nil {
		s.ToolRules = map[string]string{}
	}
	s.ToolRules[tool] = rule
	// 旧的 ask_tools/deny_tools 与新规则冲突时以新规则为准
	s.DenyTools = removeString(s.DenyTools, tool)
	if rule == "deny" {
		s.AskTools = removeString(s.AskTools, tool)
	} else if rule == "ask" {
		s.DenyTools = removeString(s.DenyTools, tool)
	} else {
		s.AskTools = removeString(s.AskTools, tool)
	}
	return nil
}

func removeString(list []string, v string) []string {
	out := list[:0]
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// ParseToolList 解析逗号分隔的工具列表。
func ParseToolList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// MaskedAPIKey 是 API 密钥在面向浏览器的快照里的占位符。真实密钥不出服务器；
// 前端原样回传该占位符时，applySettings 用旧值还原，用户填新值则正常保存。
const MaskedAPIKey = "********"

// Masked 返回密钥已替换为 MaskedAPIKey 的快照（深拷贝，安全下发到浏览器）。
func (s Settings) Masked() Settings {
	out := s
	if out.APIKey != "" {
		out.APIKey = MaskedAPIKey
	}
	if len(out.Providers) > 0 {
		out.Providers = copyProviders(out.Providers)
		for i := range out.Providers {
			if out.Providers[i].APIKey != "" {
				out.Providers[i].APIKey = MaskedAPIKey
			}
		}
	}
	return out
}

// RestoreMaskedKeys 把 prev 里的真实密钥填回本设置中仍是 MaskedAPIKey 占位符的
// 字段（顶层 + 按 Provider 名匹配）。新填的密钥不受影响。
func (s *Settings) RestoreMaskedKeys(prev Settings) {
	if s.APIKey == MaskedAPIKey {
		s.APIKey = prev.APIKey
	}
	for i := range s.Providers {
		if s.Providers[i].APIKey != MaskedAPIKey {
			continue
		}
		s.Providers[i].APIKey = ""
		for _, p := range prev.Providers {
			if p.Provider == s.Providers[i].Provider {
				s.Providers[i].APIKey = p.APIKey
				break
			}
		}
	}
}
