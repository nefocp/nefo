// All requests use paths relative to the page (no leading slash), so they go
// to /<entrance>/api/... on a server and to /api/... on the Vite dev server.

export class ApiError extends Error {
  constructor(status, code, data) {
    super(code)
    this.status = status
    this.code = code
    this.data = data
  }
}

let onUnauthorized = () => {}

export function setUnauthorizedHandler(fn) {
  onUnauthorized = fn
}

export async function api(method, path, body) {
  const headers = { 'X-Requested-With': 'panel' } // required by the CSRF check
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  let res
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, 'network', null)
  }

  let data = null
  try {
    data = await res.json()
  } catch {
    // empty or non-JSON body
  }
  if (!res.ok) {
    const code = data?.error || 'unknown'
    if (res.status === 401 && path !== 'api/auth/login' && path !== 'api/auth/me') onUnauthorized()
    throw new ApiError(res.status, code, data)
  }
  return data
}
