// Package dnsclient 提供可自定义的 DNS 解析，支持多服务器容灾、任意厂商自由填写。
// 不使用系统 DNS：只通过本包配置的服务器解析（普通 DNS (UDP/TCP)、DoT、DoH）。
package dnsclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Mode 是 DNS 解析模式。
type Mode string

const (
	ModePlain Mode = "plain" // 普通 DNS (UDP/TCP)，Server 形如 "8.8.8.8:53"
	ModeDoT   Mode = "dot"   // DNS over TLS，Server 形如 "1.1.1.1:853"
	ModeDoH   Mode = "doh"   // DNS over HTTPS，Server 形如 "https://1.1.1.1/dns-query"
)

// Server 是一个 DNS 服务器条目。
type Server struct {
	Mode   Mode   `json:"mode"`
	Server string `json:"server"`
}

// Config 描述自定义 DNS 配置。
// Mode 支持三种取值：
//   - "system"：使用系统 DNS 解析（net.DefaultResolver）
//   - "custom"：仅用 Servers 列表解析（并发竞速）
//   - "command"：通过系统命令（getent/ping/nslookup）获取 IP
//
// 空 Mode 为兼容旧配置的自动逻辑：有 Servers 视为 custom，否则并发查询内置预设。
// HostOverrides 是域名 → 指定 IP 的静态映射（如被 DNS 劫持的 API 域名），
// 命中后跳过 DNS 查询直接返回该 IP（TLS 的 SNI/证书校验仍用原域名）。
type Config struct {
	Mode          string            `json:"mode,omitempty"`
	Servers       []Server          `json:"servers"`
	Concurrency   int               `json:"concurrency,omitempty"` // 并发查询服务器数（0=默认 2，-1=全部）
	TimeoutMS     int               `json:"timeout_ms,omitempty"`  // 单次查询超时毫秒（0=默认 5000）
	HostOverrides map[string]string `json:"host_overrides,omitempty"`
}

// Presets 是常见 DNS 厂商的预设，供前端快速添加。国内厂商优先。
var Presets = map[string]Server{
	// 国内 - 阿里
	"alidns":      {Mode: ModeDoH, Server: "https://dns.alidns.com/dns-query"},
	"alidns-plain": {Mode: ModePlain, Server: "223.5.5.5:53"},
	"alidns2":     {Mode: ModePlain, Server: "223.6.6.6:53"},
	"alidns-tls":  {Mode: ModeDoT, Server: "223.5.5.5:853"},
	// 国内 - 腾讯 DNSPod
	"dnspod":      {Mode: ModeDoH, Server: "https://doh.pub/dns-query"},
	"dnspod2":     {Mode: ModePlain, Server: "119.28.28.28:53"},
	"dnspod-tls":  {Mode: ModeDoT, Server: "119.29.29.29:853"},
	// 国内 - 其他
	"onedns":      {Mode: ModeDoH, Server: "https://doh.onedns.net/dns-query"},
	"onedns-tls":  {Mode: ModeDoT, Server: "1.2.4.8:853"},
	"360":         {Mode: ModePlain, Server: "101.226.4.6:53"},
	"360-mobile":  {Mode: ModePlain, Server: "218.30.118.6:53"},
	"114":         {Mode: ModePlain, Server: "114.114.114.114:53"},
	"114-2":       {Mode: ModePlain, Server: "114.114.115.115:53"},
	"baidu":       {Mode: ModeDoH, Server: "https://doh.baidu.com/dns-query"},
	"cnnic":       {Mode: ModeDoH, Server: "https://doh.cnnic.cn/dns-query"},
	// 国外 - Cloudflare
	"cloudflare":     {Mode: ModeDoH, Server: "https://1.1.1.1/dns-query"},
	"cloudflare2":    {Mode: ModePlain, Server: "1.0.0.1:53"},
	"cloudflare-tls": {Mode: ModeDoT, Server: "1.1.1.1:853"},
	"cloudflare-fam": {Mode: ModeDoH, Server: "https://security.cloudflare-dns.com/dns-query"},
	// 国外 - Google
	"google":     {Mode: ModeDoH, Server: "https://dns.google/dns-query"},
	"google-tls": {Mode: ModeDoT, Server: "8.8.8.8:853"},
	"google2":    {Mode: ModePlain, Server: "8.8.4.4:53"},
	// 国外 - 其他
	"quad9":      {Mode: ModeDoH, Server: "https://dns.quad9.net/dns-query"},
	"quad9-tls":  {Mode: ModeDoT, Server: "9.9.9.9:853"},
	"quad9-2":    {Mode: ModePlain, Server: "9.9.9.10:53"},
	"opendns":    {Mode: ModeDoH, Server: "https://doh.opendns.com/dns-query"},
	"opendns-2":  {Mode: ModePlain, Server: "208.67.222.222:53"},
	"adguard":    {Mode: ModeDoH, Server: "https://dns.adguard-dns.com/dns-query"},
	"adguard-tls": {Mode: ModeDoT, Server: "94.140.14.14:853"},
	"adguard-2":  {Mode: ModePlain, Server: "94.140.14.15:53"},
	"nextdns":    {Mode: ModeDoH, Server: "https://dns.nextdns.io/dns-query"},
	"mullvad":    {Mode: ModeDoH, Server: "https://dns.mullvad.net/dns-query"},
	"control-d":  {Mode: ModeDoH, Server: "https://freedns.controld.com/p0"},
}

// Defaults 是内置的默认 DNS 配置：阿里主 + 腾讯备（普通 UDP）。
func Defaults() Config {
	return Config{
		Servers: []Server{
			{Mode: ModePlain, Server: "223.5.5.5:53"},
			{Mode: ModePlain, Server: "119.29.29.29:53"},
		},
	}
}

// keyOf 反查 Presets 的键名（按 server 值匹配）。
func keyOf(s Server) string {
	for k, v := range Presets {
		if v.Server == s.Server {
			return k
		}
	}
	return ""
}

// presetOrder 是预设的展示/兜底顺序（国内优先，其次国外）。
var presetOrder = []string{
	"alidns", "alidns-plain", "alidns2", "alidns-tls",
	"dnspod", "dnspod2", "dnspod-tls",
	"onedns", "onedns-tls", "360", "360-mobile", "114", "114-2", "baidu", "cnnic",
	"cloudflare", "cloudflare2", "cloudflare-tls", "cloudflare-fam",
	"google", "google2", "google-tls",
	"quad9", "quad9-2", "quad9-tls",
	"opendns", "opendns-2",
	"adguard", "adguard-2", "adguard-tls",
	"nextdns", "mullvad", "control-d",
}

// allPresetServers 返回全部内置预设服务器（国内外、plain/DoT/DoH 全覆盖）。
// 用户一条 DNS 都没配时，用它们并发查询。
func allPresetServers() []Server {
	rank := map[string]int{}
	for i, k := range presetOrder {
		rank[k] = i
	}
	out := make([]Server, 0, len(Presets))
	for k, s := range Presets {
		if _, ok := rank[k]; !ok {
			rank[k] = len(presetOrder)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return rank[keyOf(out[i])] < rank[keyOf(out[j])] })
	return out
}

func (c Config) isCustom() bool {
	return len(c.activeServers()) > 0
}

func lookupPlain(ctx context.Context, server, host string) ([]net.IP, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := &net.Dialer{Timeout: 8 * time.Second}
			return d.DialContext(ctx, network, server)
		},
	}
	return r.LookupIP(ctx, "ip", host)
}

// ---- DNS wire format ----

type dnsWire struct {
	data []byte
	pos  int
}

func newDNSQuery(domain string, qtype uint16) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(0x1234))
	binary.Write(&buf, binary.BigEndian, uint16(0x0100))
	binary.Write(&buf, binary.BigEndian, uint16(1))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			continue
		}
		// 边界：DNS 标签长度是 1 字节（<=63 为规范值）。不加限制时超长子域
		// 会把长度字节溢出成别的值，发出畸形查询、解析器按错位继续读响应。
		if len(label) > 63 {
			return nil
		}
		buf.WriteByte(byte(len(label)))
		buf.WriteString(label)
	}
	if len(domain) > 253 {
		return nil
	}
	buf.WriteByte(0)
	binary.Write(&buf, binary.BigEndian, qtype)
	binary.Write(&buf, binary.BigEndian, uint16(1))
	return buf.Bytes()
}

func (w *dnsWire) skipName() {
	for {
		if w.pos >= len(w.data) {
			w.pos = len(w.data)
			return
		}
		b := w.data[w.pos]
		if b == 0 {
			w.pos++
			return
		}
		if b&0xc0 == 0xc0 {
			w.pos += 2
			if w.pos > len(w.data) {
				w.pos = len(w.data)
			}
			return
		}
		w.pos += int(b) + 1
	}
}

func (w *dnsWire) readUint16() (uint16, bool) {
	if w.pos+2 > len(w.data) {
		return 0, false
	}
	v := binary.BigEndian.Uint16(w.data[w.pos : w.pos+2])
	w.pos += 2
	return v, true
}

func (w *dnsWire) readUint32() (uint32, bool) {
	if w.pos+4 > len(w.data) {
		return 0, false
	}
	v := binary.BigEndian.Uint32(w.data[w.pos : w.pos+4])
	w.pos += 4
	return v, true
}

// parseDNSResponse 解析 DNS 响应中的 A/AAAA 记录。
// 响应来自网络（DoH 允许明文 http），任何位置都必须先做边界检查，
// 否则畸形/被劫持的响应会导致切片越界 panic 崩掉整个进程。
func parseDNSResponse(data []byte, qtype uint16) ([]net.IP, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("DNS 响应过短")
	}
	w := &dnsWire{data: data, pos: 12}
	w.skipName()
	w.pos += 4 // QTYPE + QCLASS
	var ips []net.IP
	ancount := binary.BigEndian.Uint16(data[6:8])
	for i := 0; i < int(ancount); i++ {
		if w.pos >= len(w.data) {
			break
		}
		w.skipName()
		rt, ok := w.readUint16()
		if !ok {
			break
		}
		if _, ok := w.readUint16(); !ok { // class
			break
		}
		if _, ok := w.readUint32(); !ok { // ttl
			break
		}
		rdlen, ok := w.readUint16()
		if !ok {
			break
		}
		if w.pos+int(rdlen) > len(w.data) {
			break
		}
		if rt == 1 && qtype == 1 && rdlen == 4 {
			ip := net.IPv4(w.data[w.pos], w.data[w.pos+1], w.data[w.pos+2], w.data[w.pos+3])
			ips = append(ips, ip)
		} else if rt == 28 && qtype == 28 && rdlen == 16 {
			ip := make(net.IP, 16)
			copy(ip, w.data[w.pos:w.pos+16])
			ips = append(ips, ip)
		}
		w.pos += int(rdlen)
	}
	return ips, nil
}

func dohQuery(ctx context.Context, server, host string, qtype uint16) ([]net.IP, error) {
	query := newDNSQuery(host, qtype)
	url := server
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	if query == nil {
		return nil, fmt.Errorf("DNS 查询名过长或非法: %s", host)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH 返回 %d", resp.StatusCode)
	}
	return parseDNSResponse(body, qtype)
}

func dotQuery(ctx context.Context, server, host string, qtype uint16) ([]net.IP, error) {
	// 用 SplitHostPort/JoinHostPort 而非字符串拼接：裸 IPv6（含冒号）
	// 会被旧逻辑误判为“已带端口”。显式配置的端口必须保留（如自建 8443 的 DoT），
	// 只有确实不带端口时才补默认 853。
	hostOnly := server
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "853")
		hostOnly = server[:strings.LastIndex(server, ":")]
	} else if h, _, e := net.SplitHostPort(server); e == nil {
		hostOnly = h
	}
	d := &net.Dialer{Timeout: 8 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	tlsConn := tls.Client(conn, &tls.Config{ServerName: hostOnly})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	query := newDNSQuery(host, qtype)
	if query == nil {
		return nil, fmt.Errorf("DNS 查询名过长或非法: %s", host)
	}
	var msg bytes.Buffer
	binary.Write(&msg, binary.BigEndian, uint16(len(query)))
	msg.Write(query)
	if _, err := tlsConn.Write(msg.Bytes()); err != nil {
		return nil, err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(tlsConn, hdr[:]); err != nil {
		return nil, err
	}
	respLen := binary.BigEndian.Uint16(hdr[:])
	resp := make([]byte, respLen)
	if _, err := io.ReadFull(tlsConn, resp); err != nil {
		return nil, err
	}
	return parseDNSResponse(resp, qtype)
}

func (srv Server) lookup(ctx context.Context, host string, qtype uint16) ([]net.IP, error) {
	switch srv.Mode {
	case ModePlain:
		return lookupPlain(ctx, srv.Server, host)
	case ModeDoT:
		return dotQuery(ctx, srv.Server, host, qtype)
	case ModeDoH:
		return dohQuery(ctx, srv.Server, host, qtype)
	default:
		// 兼容旧配置里的 "system" 模式：没有实际服务器，跳过。
		return nil, fmt.Errorf("不支持的 DNS 模式 %q", srv.Mode)
	}
}

// LookupIP 解析 host 的 IP 列表。
// 静态映射优先；随后按 Mode（system/custom/command）选择解析方式；
// 旧配置（Mode 为空）保持原行为：有服务器用服务器，否则并发查询全部内置预设。
func (c Config) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSuffix(host, ".")
	// 静态映射优先：域名被劫持/污染时，用户可为该域名指定真实 IP。
	if ipStr := strings.TrimSpace(c.HostOverrides[strings.ToLower(host)]); ipStr != "" {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("DNS 解析 %s 失败: 指定 IP %q 无效", host, ipStr)
		}
		return []net.IP{ip}, nil
	}
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case "system":
		return systemLookup(ctx, host)
	case "command":
		return commandLookup(ctx, host)
	case "custom":
		servers := c.activeServers()
		if len(servers) == 0 {
			return nil, fmt.Errorf("DNS 解析 %s 失败: 自定义模式未配置服务器", host)
		}
		return c.lookupAll(ctx, host)
	}
	servers := c.activeServers()
	if len(servers) == 0 {
		// 未配置任何 DNS：并发查询所有预设（国内外 plain/DoT/DoH 全部），取最快成功结果。
		cfg := Config{Servers: allPresetServers(), Concurrency: 0, TimeoutMS: c.TimeoutMS}
		return cfg.lookupAll(ctx, host)
	}
	return c.lookupAll(ctx, host)
}

// systemLookup 使用系统 DNS 解析。
func systemLookup(ctx context.Context, host string) ([]net.IP, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("DNS 解析 %s 失败（系统 DNS）: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("DNS 解析 %s 失败（系统 DNS 无结果）", host)
	}
	return ips, nil
}

// hostNamePattern 限制可传给系统命令的主机名，防命令注入。
var hostNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// commandLookup 通过系统命令获取 IP：getent → ping → nslookup，
// 全部不可用时回退系统 DNS，保证可用性。
func commandLookup(ctx context.Context, host string) ([]net.IP, error) {
	if !hostNamePattern.MatchString(host) {
		return nil, fmt.Errorf("DNS 解析失败: 非法主机名 %q", host)
	}
	if _, err := exec.LookPath("getent"); err == nil {
		if out, err := exec.CommandContext(ctx, "getent", "ahosts", host).CombinedOutput(); err == nil {
			if ips := parseIPsFromText(string(out)); len(ips) > 0 {
				return ips, nil
			}
		}
	}
	if _, err := exec.LookPath("ping"); err == nil {
		// Linux/Termux: ping -c 1 -W 2；macOS/BSD 的 -W 语义不同，失败则退回无 -W。
		out, err := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", host).CombinedOutput()
		if err != nil {
			out, _ = exec.CommandContext(ctx, "ping", "-c", "1", host).CombinedOutput()
		}
		if ips := parseIPsFromText(string(out)); len(ips) > 0 {
			return ips, nil
		}
	}
	if _, err := exec.LookPath("nslookup"); err == nil {
		if out, err := exec.CommandContext(ctx, "nslookup", host).CombinedOutput(); err == nil {
			if ips := parseIPsFromText(string(out)); len(ips) > 0 {
				return ips, nil
			}
		}
	}
	return systemLookup(ctx, host)
}

// ipTextPattern 匹配文本中的 IPv4/IPv6 字面量。
var ipTextPattern = regexp.MustCompile(`(?:[0-9]{1,3}\.){3}[0-9]{1,3}|[0-9a-fA-F]{0,4}(?::[0-9a-fA-F]{0,4}){2,7}`)

// parseIPsFromText 从命令输出中提取合法 IP（去重、保序）。
func parseIPsFromText(s string) []net.IP {
	var out []net.IP
	seen := map[string]bool{}
	for _, m := range ipTextPattern.FindAllString(s, -1) {
		ip := net.ParseIP(strings.Trim(m, "[]"))
		if ip == nil {
			continue
		}
		key := ip.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ip)
	}
	return out
}

// queryTimeout 返回单次查询超时。
func (c Config) queryTimeout() time.Duration {
	if c.TimeoutMS > 0 {
		return time.Duration(c.TimeoutMS) * time.Millisecond
	}
	return 5 * time.Second
}

// queryLimit 返回并发查询的服务器数量上限。Concurrency=-1 表示全部并发。
func (c Config) queryLimit() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return 2 // 默认：主 + 备同时查，取最快
}

// lookupAll 用前 queryLimit 个服务器并发解析。
// 竞速语义：哪个 DNS 先返回可用 IP 就立即采用哪个的结果，不再等其余服务器。
// 一个都没成功时，等所有结果返回后报错。
func (c Config) lookupAll(ctx context.Context, host string) ([]net.IP, error) {
	servers := c.activeServers()
	limit := c.queryLimit()
	if limit > 0 && len(servers) > limit {
		servers = servers[:limit]
	}
	qctx, cancel := context.WithTimeout(ctx, c.queryTimeout())
	defer cancel()

	type result struct {
		ips []net.IP
		err error
	}
	resCh := make(chan result, len(servers))
	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(srv Server) {
			defer wg.Done()
			ips, err := srv.lookup(qctx, host, 1)
			if (err != nil || len(ips) == 0) && qctx.Err() == nil {
				// A 记录失败再试 AAAA，尽量拿到可用结果
				ips6, err6 := srv.lookup(qctx, host, 28)
				if err6 == nil && len(ips6) > 0 {
					resCh <- result{ips6, nil}
					return
				}
			}
			resCh <- result{ips, err}
		}(servers[i])
	}
	go func() { wg.Wait(); close(resCh) }()

	var firstErr error
	failed := 0
	for r := range resCh {
		if r.err == nil && len(r.ips) > 0 {
			// 竞速：第一个返回 IP 的服务器胜出，立即返回
			return r.ips, nil
		}
		failed++
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
	}
	if failed == len(servers) {
		if firstErr != nil {
			return nil, fmt.Errorf("DNS 解析 %s 失败: %w", host, firstErr)
		}
		return nil, fmt.Errorf("DNS 解析 %s 失败: 所有 DNS 服务器均未返回结果", host)
	}
	return nil, fmt.Errorf("DNS 解析 %s 失败: 无可用结果", host)
}

func (c Config) activeServers() []Server {
	var out []Server
	for _, s := range c.Servers {
		if s.Mode != "" && strings.TrimSpace(s.Server) != "" {
			out = append(out, s)
		}
	}
	return out
}

// Resolver 返回一个可复用的拨号函数，供 http.Transport.DialContext 使用。
// system 模式返回 nil（交给系统解析）；custom/command 模式返回自定义拨号；
// 旧配置（Mode 为空）无服务器时也返回 nil。
func (c Config) Resolver() func(ctx context.Context, network, address string) (net.Conn, error) {
	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	if mode == "system" {
		return nil
	}
	if mode == "" && !c.isCustom() {
		return nil
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			host = address
			port = "443"
		}
		if host == "" {
			return nil, fmt.Errorf("DNS 解析: 空地址")
		}
		ips, err := c.LookupIP(ctx, host)
		if err != nil {
			return nil, err
		}
		d := &net.Dialer{Timeout: 15 * time.Second}
		var lastErr error
		for _, ip := range ips {
			conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("DNS 解析 %s: 无可用地址", host)
	}
}
