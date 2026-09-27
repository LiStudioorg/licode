#!/usr/bin/env node
/**
 * 本地 mock LLM 端点（OpenAI 兼容），用于在无外网/无 API key 时验证
 * Licode 全链路：流式输出 -> 工具调用 -> 审批 -> 结果回填。
 *
 * 用法：
 *   node scripts/mock-llm.mjs          # 监听 127.0.0.1:11434
 *
 * 然后在 Licode 设置里把 provider 指向它：
 *   base_url = http://127.0.0.1:11434/v1
 *   api_key  = mock
 *   model    = mock-agent
 *
 * 行为设计：
 *   - 第一轮：返回一个 Shell 工具调用（触发审批，因为默认 Shell=ask）
 *   - 收到 tool 结果后：流式输出最终答复
 * 这样一次对话就能走完「流式 + 审批 + 回填」。
 */
import http from 'node:http'

const PORT = Number(process.env.MOCK_PORT ?? 11434)
const HOST = '127.0.0.1'

/**
 * 是否已经请求过工具由「对话里是否已有 tool 结果 / assistant tool_calls」判断，
 * 因此无需在服务端保留状态，多次测试互不污染。
 */

function sse(res, obj) {
  res.write(`data: ${JSON.stringify(obj)}\n\n`)
}

function chunk(delta) {
  return {
    id: 'chatcmpl-mock',
    object: 'chat.completion.chunk',
    created: Math.floor(Date.now() / 1000),
    model: 'mock-agent',
    choices: [{ index: 0, delta, finish_reason: null }],
  }
}

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url.startsWith('/v1/models')) {
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ object: 'list', data: [{ id: 'mock-agent', object: 'model' }] }))
    return
  }

  if (req.method !== 'POST' || !req.url.startsWith('/v1/chat/completions')) {
    res.writeHead(404).end('not found')
    return
  }

  let body = ''
  req.on('data', (c) => (body += c))
  req.on('end', () => {
    let payload = {}
    try {
      payload = JSON.parse(body)
    } catch {
      /* 忽略坏请求体 */
    }
    const messages = Array.isArray(payload.messages) ? payload.messages : []
    // 已经有 tool 结果回填 => 进入收尾轮
    const hasToolResult = messages.some((m) => m.role === 'tool')
    const hasToolCall = messages.some((m) => m.role === 'assistant' && m.tool_calls?.length)

    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive',
    })

    if (!hasToolResult && !hasToolCall) {
      // 第一轮：发起 Shell 调用，触发审批流程
      const args = JSON.stringify({ command: 'echo licode-mock-ok' })
      sse(res, chunk({ role: 'assistant', content: '我先执行一条命令确认环境。' }))
      sse(res, chunk({
        tool_calls: [{
          index: 0,
          id: 'call_mock_1',
          type: 'function',
          function: { name: 'Shell', arguments: args },
        }],
      }))
      sse(res, { ...chunk({}), choices: [{ index: 0, delta: {}, finish_reason: 'tool_calls' }] })
      res.write('data: [DONE]\n\n')
      res.end()
      console.log('[mock] 第一轮：已发出 Shell 工具调用，等待审批')
      return
    }

    // 收尾轮：流式输出最终答复，分片模拟真实打字
    const text = hasToolResult
      ? '命令已执行完成，链路验证通过：流式输出、工具审批、结果回填全部正常。'
      : '这是一次 mock 回复，用于验证流式渲染。'
    const parts = text.match(/.{1,6}/gu) ?? [text]
    let i = 0
    const timer = setInterval(() => {
      if (i >= parts.length) {
        clearInterval(timer)
        sse(res, { ...chunk({}), choices: [{ index: 0, delta: {}, finish_reason: 'stop' }] })
        res.write('data: [DONE]\n\n')
        res.end()
        console.log('[mock] 收尾轮：流式输出完成')
        return
      }
      sse(res, chunk({ content: parts[i] }))
      i += 1
    }, 60)
  })
})

server.listen(PORT, HOST, () => {
  console.log(`mock LLM 已启动: http://${HOST}:${PORT}/v1  (model=mock-agent)`)
})
