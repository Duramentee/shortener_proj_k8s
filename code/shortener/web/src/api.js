// 后端接口的访问层。本文件是前端唯一发起 HTTP 请求的位置，
// 组件只调用这里导出的函数，不直接使用 fetch，这样接口路径与错误处理逻辑只会出现在一个地方。
//
// 本文件实现的函数与 code/shortener/README.md 第 4 节的接口契约逐条对应：
//   createLink     对应 POST   /api/links
//   listLinks      对应 GET    /api/links
//   deleteLink     对应 DELETE /api/links/{code}
//   fetchStats     对应 GET    /api/stats
//   fetchReadiness 对应 GET    /api/readyz
//
// 所有请求都使用相对路径（例如 /api/links 而不是 http://api:8080/api/links），
// 因为部署形态下由 nginx 把 /api/ 前缀反向代理给后端，
// 开发形态下由 Vite 的 server.proxy 转发给后端，两种形态下前端代码保持一致。

/** 请求超时时间，单位是毫秒。超过这个时间没有响应就中断请求，避免界面长时间停留在加载状态。 */
const REQUEST_TIMEOUT_MS = 10000

/**
 * 把结果解析为普通对象。后端的成功响应都是 JSON，
 * 但是 DELETE 接口返回 204 并且没有响应体，因此这里需要单独判断。
 */
async function readJsonBody(response) {
  if (response.status === 204) {
    return null
  }
  const text = await response.text()
  if (text === '') {
    return null
  }
  try {
    return JSON.parse(text)
  } catch (parseError) {
    return null
  }
}

/**
 * 从失败响应中提取错误信息。
 * 按照接口契约，后端在出错时返回 {"error":"..."} 这样的结构，
 * 因此优先使用这个字段的内容；如果响应体不是这种结构，就用 HTTP 状态码拼出一句可读的提示。
 */
async function extractErrorMessage(response, fallbackBody) {
  if (fallbackBody && typeof fallbackBody.error === 'string' && fallbackBody.error !== '') {
    return fallbackBody.error
  }
  return `请求失败，HTTP 状态码是 ${response.status}`
}

/**
 * 统一发起请求并且统一处理错误。
 * @param {string} path 请求路径，例如 /api/links
 * @param {RequestInit} options fetch 的第二个参数
 * @returns {Promise<any>} 解析后的响应体；204 响应返回 null
 */
async function request(path, options = {}) {
  const init = { ...options }

  // 只有在浏览器支持 AbortSignal.timeout 时才附加超时信号，避免在不支持该特性的环境里抛异常。
  if (init.signal === undefined && typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
    init.signal = AbortSignal.timeout(REQUEST_TIMEOUT_MS)
  }

  let response
  try {
    response = await fetch(path, init)
  } catch (networkError) {
    // 网络层错误包括：后端进程没有启动、反向代理无法解析上游主机名、请求超时。
    // 这些情况下浏览器不会给出 HTTP 状态码，因此必须在这里单独给出提示。
    throw new Error('无法连接到后端服务，请确认 api 服务正在运行，并且反向代理的上游地址配置正确')
  }

  const body = await readJsonBody(response)

  if (!response.ok) {
    throw new Error(await extractErrorMessage(response, body))
  }

  return body
}

/**
 * 创建一个短链接。
 * @param {string} url 用户提交的原始长网址
 * @returns {Promise<{code: string, url: string, clicks: number, createdAt: string}>}
 */
export function createLink(url) {
  return request('/api/links', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url })
  })
}

/**
 * 按创建时间倒序分页列出短链接。
 * @param {{limit?: number, offset?: number}} params 分页参数
 * @returns {Promise<{items: Array<object>, total: number, limit: number, offset: number}>}
 */
export function listLinks({ limit = 20, offset = 0 } = {}) {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  return request(`/api/links?${query.toString()}`)
}

/**
 * 删除一条短链接。后端在删除成功时返回 204，因此本函数的返回值是 null。
 * @param {string} code 六位短码
 */
export function deleteLink(code) {
  return request(`/api/links/${encodeURIComponent(code)}`, { method: 'DELETE' })
}

/**
 * 获取汇总统计。
 * @returns {Promise<{links: number, clicks: number, cacheHits: number, cacheMisses: number, cacheHitRate: number}>}
 */
export function fetchStats() {
  return request('/api/stats')
}

/**
 * 查询后端就绪状态。
 *
 * 这个函数与上面几个函数的区别是：它不会把 503 当作异常抛出。
 * 原因是就绪探针的语义本来就是「未就绪时返回 503」，而前端需要读取响应体里的 checks 字段
 * 才能显示出到底是 PostgreSQL 不可用还是 Redis 不可用，因此这里必须把状态码与响应体一起返回。
 *
 * @returns {Promise<{ok: boolean, status: number, body: object|null}>}
 */
export async function fetchReadiness() {
  let response
  try {
    response = await fetch('/api/readyz', { headers: { Accept: 'application/json' } })
  } catch (networkError) {
    // status 为 0 表示连 HTTP 响应都没有拿到，界面上据此显示「无法连接后端」。
    return { ok: false, status: 0, body: null }
  }

  const body = await readJsonBody(response)
  return { ok: response.ok, status: response.status, body }
}
