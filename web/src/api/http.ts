/**
 * REST 客户端。
 *
 * 后端的错误约定并不完全统一（部分端点 401 返回纯文本、部分返回 JSON
 * {error}），这里统一成抛 Error(message)：
 *   - 401 一律跳转 /login（未启用登录时后端不会返回 401）；
 *   - 非 2xx 优先取 JSON.error，退化为状态码文本。
 */

export class ApiError extends Error {
  readonly status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/**
 * 统一 401 处理：整页跳登录页。
 *
 * 不用 router.push 是为了避开 router → views → stores → http 的循环依赖；
 * 会话过期是低频事件，整页刷新的代价可接受，且能顺带清掉失效的内存状态。
 * 后端未登录时若请求缺 Accept: application/json 会返回 302 的 HTML，
 * 因此本模块所有请求都显式带该头，确保拿到干净的 401。
 */
function onUnauthorized(): void {
  if (location.pathname === '/login') return
  location.assign('/login')
}

async function toError(res: Response): Promise<ApiError> {
  let msg = `请求失败 (${res.status})`
  try {
    const text = await res.text()
    if (text) {
      try {
        const j = JSON.parse(text) as { error?: string }
        if (j.error) msg = j.error
      } catch {
        // 纯文本错误（如 401「未登录」）直接用原文
        if (text.length < 200) msg = text
      }
    }
  } catch {
    /* 忽略读取失败 */
  }
  return new ApiError(msg, res.status)
}

/** GET JSON */
export async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' }, credentials: 'same-origin' })
  if (res.status === 401) {
    onUnauthorized()
    throw new ApiError('未登录', 401)
  }
  if (!res.ok) throw await toError(res)
  return (await res.json()) as T
}

/** POST JSON（body 为对象时序列化；传 FormData 时交给浏览器带 boundary） */
export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method: 'POST',
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
  }
  if (body instanceof FormData) {
    init.body = body
  } else if (body !== undefined) {
    init.headers = { ...init.headers, 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }
  const res = await fetch(path, init)
  if (res.status === 401) {
    onUnauthorized()
    throw new ApiError('未登录', 401)
  }
  if (!res.ok) throw await toError(res)
  // 部分端点返回空体
  const text = await res.text()
  if (!text) return undefined as T
  try {
    return JSON.parse(text) as T
  } catch {
    return text as unknown as T
  }
}

/** POST 原始字节（备份导入：body 是 zip 二进制） */
export async function apiPostRaw<T>(path: string, data: Blob | ArrayBuffer): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { Accept: 'application/json', 'Content-Type': 'application/octet-stream' },
    body: data,
  })
  if (res.status === 401) {
    onUnauthorized()
    throw new ApiError('未登录', 401)
  }
  if (!res.ok) throw await toError(res)
  return (await res.json()) as T
}

/** 触发浏览器下载（后端已带 Content-Disposition） */
export function downloadUrl(path: string, filename?: string): void {
  const a = document.createElement('a')
  a.href = path
  if (filename) a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
}

/** 取回文本内容（导出会话 md） */
export async function fetchText(path: string): Promise<string> {
  const res = await fetch(path, { credentials: 'same-origin' })
  if (res.status === 401) {
    onUnauthorized()
    throw new ApiError('未登录', 401)
  }
  if (!res.ok) throw await toError(res)
  return res.text()
}
