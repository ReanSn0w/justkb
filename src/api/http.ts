import type { ErrorEnvelope } from './types'

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details: Record<string, unknown> = {},
    public requestId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

let csrfToken = ''
export function setCsrfToken(value: string) {
  csrfToken = value
}

async function decodeError(response: Response): Promise<ApiError> {
  let envelope: ErrorEnvelope | undefined
  try {
    envelope = (await response.json()) as ErrorEnvelope
  } catch {
    /* non-JSON proxy response */
  }
  return new ApiError(
    response.status,
    envelope?.error.code || 'http_error',
    envelope?.error.message || `HTTP ${response.status}`,
    envelope?.error.details,
    envelope?.error.request_id || response.headers.get('x-request-id') || undefined,
  )
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method || 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData)) headers.set('content-type', 'application/json')
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && csrfToken)
    headers.set('x-csrf-token', csrfToken)
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    method,
    headers,
    credentials: 'include',
  })
  if (!response.ok) throw await decodeError(response)
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

export function json(method: string, body?: unknown): RequestInit {
  return { method, body: body === undefined ? undefined : JSON.stringify(body) }
}

export function upload<T>(
  path: string,
  file: File,
  onProgress?: (loaded: number, total: number) => void,
): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `/api/v1${path}`)
    xhr.withCredentials = true
    if (csrfToken) xhr.setRequestHeader('x-csrf-token', csrfToken)
    xhr.upload.onprogress = (event) => onProgress?.(event.loaded, event.total)
    xhr.onerror = () => reject(new ApiError(0, 'network_error', 'Не удалось загрузить файл'))
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as T)
        } catch {
          reject(new ApiError(xhr.status, 'invalid_response', 'Некорректный ответ сервера'))
        }
      } else {
        try {
          const envelope = JSON.parse(xhr.responseText) as ErrorEnvelope
          reject(
            new ApiError(
              xhr.status,
              envelope.error.code,
              envelope.error.message,
              envelope.error.details,
            ),
          )
        } catch {
          reject(new ApiError(xhr.status, 'http_error', `HTTP ${xhr.status}`))
        }
      }
    }
    const form = new FormData()
    form.append('file', file)
    xhr.send(form)
  })
}
