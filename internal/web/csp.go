package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// CSP 收紧：script-src 不再放行 'unsafe-inline'，改为对 go:embed 的前端
// 产物做一次性扫描，为其中每个内联 <script> 计算 sha256 哈希白名单。
// 产物是编译期固化的嵌入文件，哈希集合恒定，因此"产物不变即零风险"，
// 而任意注入出的新内联脚本（XSS 载荷的形态）会被浏览器直接拒绝执行。
// 重新生成前端（LICODE_SYNC_NUXT=1）后哈希自动跟随新产物，无需人工维护。
//
// 已知权衡：若未来某个页面的内联脚本由前端运行时动态改写（eval 注入），
// 将不再执行——那种前端本身与任何 CSP 都互斥，属于需要重构的信号。
var (
	scriptTagRe = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	jsonTypeRe  = regexp.MustCompile(`(?i)type\s*=\s*["']?[^\s"']*json`)
	srcAttrRe   = regexp.MustCompile(`(?i)\bsrc\s*=`)

	cspOnce      sync.Once
	cspScriptSrc string
)

// ScriptSrcCSP 返回 script-src 的值（进程内计算一次）。嵌入产物里没有任何
// 内联脚本时返回 "'self'"；扫描失败时保守返回 "'self' 'unsafe-inline'"
// （宁可维持旧行为，也不产生白屏事故——失败路径只影响 CSP 收紧与否）。
func ScriptSrcCSP() string {
	cspOnce.Do(func() {
		hashes := map[string]bool{}
		var anyInline bool
		err := fs.WalkDir(staticFS, "dist", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.ToLower(filepath.Ext(p)) != ".html" {
				return nil
			}
			data, rerr := fs.ReadFile(staticFS, p)
			if rerr != nil {
				return nil
			}
			for _, tag := range scriptTagRe.FindAllSubmatch(data, -1) {
				attrs, body := bytes.ToLower(tag[1]), tag[2]
				if srcAttrRe.Match(attrs) || jsonTypeRe.Match(attrs) || len(bytes.TrimSpace(body)) == 0 {
					continue
				}
				anyInline = true
				sum := sha256.Sum256(body)
				hashes["'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'"] = true
			}
			return nil
		})
		if err != nil {
			cspScriptSrc = "'self' 'unsafe-inline'"
			return
		}
		list := make([]string, 0, len(hashes))
		for h := range hashes {
			list = append(list, h)
		}
		sort.Strings(list) // 头部稳定，便于缓存与测试断言
		cspScriptSrc = "'self'"
		switch {
		case len(list) > 0:
			cspScriptSrc += " " + strings.Join(list, " ")
		case anyInline:
			// 产物里有内联脚本却一个哈希都没算出来：解析异常，保守回退
			// 旧行为（宁可 CSP 松，不可白屏）。产物确无内联脚本时维持严格。
			cspScriptSrc = "'self' 'unsafe-inline'"
		}
	})
	return cspScriptSrc
}
