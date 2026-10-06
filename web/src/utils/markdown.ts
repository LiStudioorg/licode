/**
 * Markdown 渲染（自研零依赖子集）。
 *
 * 为什么不引第三方 markdown 库：
 *   - 产物 go:embed 进二进制，每个 KB 都是发布资产体积；
 *   - 后端 CSP 严格（script-src 'self'+hash），任何库的运行时内联都是风险面；
 *   - LLM 输出 95% 落在有限子集内：围栏代码/行内码/粗斜体/链接/标题/
 *     有序无序列表/引用/水平线/段落。
 *
 * 安全模型（必须保持）：
 *   1. 代码块先抽出为占位符（\u0000CODEn\u0000），其余内容整体 HTML 转义，
 *      所有再注入的标签都由本文件产生 —— 源文本永远无法携带可执行标签；
 *   2. 链接仅放行 http/https 协议且 URL 整体转义，杜绝 javascript: 注入；
 *   3. 输出只绑定到 v-html 的消息体，勿用于属性上下文。
 */

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/** 行内语法：输入必须是已转义文本 */
function inline(escaped: string): string {
  return escaped
    .replace(/`([^`\n]+)`/g, '<code class="md-code">$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*\w])\*([^*\n]+)\*(?!\*)/g, '$1<em>$2</em>')
    .replace(
      /\[([^\]]+)\]\((https?:[^)\s]+)\)/g,
      '<a href="$2" target="_blank" rel="noopener noreferrer" class="md-link">$1</a>',
    )
}

interface CodeBlock {
  lang: string
  code: string
}

function renderCodeBlock(c: CodeBlock): string {
  const lang = escapeHtml(c.lang || 'text')
  const enc = encodeURIComponent(c.code)
  return (
    `<div class="md-pre" data-lang="${lang}">` +
    `<div class="md-pre-head"><span>${lang}</span>` +
    `<button type="button" class="md-copy" data-code="${enc}" aria-label="复制代码">复制</button></div>` +
    `<pre><code>${escapeHtml(c.code)}</code></pre></div>`
  )
}

export function renderMarkdown(src: string): string {
  if (!src) return ''

  // 1. 抽围栏代码块（``` 与 ~~~），避免其内容被行内语法污染
  const codes: CodeBlock[] = []
  let text = src.replace(/(?:```|~~~)([+\-\w]*)[ \t]*\n?([\s\S]*?)(?:```|~~~)[ \t]*\n?/g, (_m, lang: string, code: string) => {
    codes.push({ lang, code: code.replace(/\n$/, '') })
    return `\u0000CODE${codes.length - 1}\u0000`
  })

  // 2. 整体转义，此后注入安全
  text = escapeHtml(text)

  // 3. 逐行块级解析
  const lines = text.split('\n')
  const out: string[] = []
  let listType: 'ul' | 'ol' | null = null
  let para: string[] = []

  const flushPara = () => {
    if (para.length) {
      out.push(`<p class="md-p">${inline(para.join('<br>'))}</p>`)
      para = []
    }
  }
  const closeList = () => {
    if (listType) {
      out.push(`</${listType}>`)
      listType = null
    }
  }

  for (const line of lines) {
    const codeMatch = line.match(/^\u0000CODE(\d+)\u0000\s*$/)
    if (codeMatch) {
      flushPara()
      closeList()
      out.push(renderCodeBlock(codes[Number(codeMatch[1])]))
      continue
    }
    if (/^\s*(---+|\*\*\*+|___+)\s*$/.test(line)) {
      flushPara()
      closeList()
      out.push('<hr class="md-hr">')
      continue
    }
    const h = line.match(/^(#{1,6})\s+(.*)$/)
    if (h) {
      flushPara()
      closeList()
      const n = Math.min(h[1].length + 1, 5)
      out.push(`<h${n} class="md-h md-h${n}">${inline(h[2])}</h${n}>`)
      continue
    }
    const quote = line.match(/^&gt;\s?(.*)$/)
    if (quote) {
      flushPara()
      closeList()
      out.push(`<blockquote class="md-quote">${inline(quote[1])}</blockquote>`)
      continue
    }
    const ul = line.match(/^\s*[-*+]\s+(.*)$/)
    const ol = line.match(/^\s*\d+[.、]\s+(.*)$/)
    if (ul || ol) {
      flushPara()
      const want = ul ? 'ul' : 'ol'
      if (listType !== want) {
        closeList()
        out.push(want === 'ul' ? '<ul class="md-list">' : '<ol class="md-list md-ol">')
        listType = want
      }
      out.push(`<li>${inline((ul ?? ol)![1])}</li>`)
      continue
    }
    if (line.trim() === '') {
      flushPara()
      closeList()
      continue
    }
    closeList()
    para.push(line)
  }
  flushPara()
  closeList()

  return out.join('')
}

/** 事件委托：处理 .md-copy 按钮的复制（供消息流挂载一个全局 click 监听） */
export function bindMarkdownClick(el: HTMLElement): void {
  el.addEventListener('click', (e) => {
    const target = e.target as HTMLElement | null
    if (!target || !target.classList.contains('md-copy')) return
    const code = decodeURIComponent(target.dataset.code ?? '')
    void navigator.clipboard
      ?.writeText(code)
      .then(() => {
        target.textContent = '已复制'
        setTimeout(() => (target.textContent = '复制'), 1200)
      })
      .catch(() => {
        target.textContent = '失败'
        setTimeout(() => (target.textContent = '复制'), 1200)
      })
  })
}
