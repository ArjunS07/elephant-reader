import type { FeedResponse, Highlight, Item, RowModel, Source, UserItemState, UserPreferences } from './types.ts'

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

function authToken(): string | undefined {
  const env = import.meta.env as Record<string, string | undefined>
  return env.VITE_ELEPHANT_API_TOKEN
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  const form = typeof FormData !== 'undefined' && init?.body instanceof FormData
  if (init?.body && !form && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const token = authToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(path, { ...init, headers })
  if (response.status === 204) return undefined as T
  const text = await response.text()
  const body = text ? (JSON.parse(text) as { error?: string }) : {}
  if (!response.ok) throw new ApiError(response.status, body.error || response.statusText)
  return body as T
}

export function health(): Promise<{ ok: boolean }> {
  return request('/v1/health')
}

export function feed(query: { day?: string; folder?: string; sourceId?: string }): Promise<FeedResponse> {
  const params = new URLSearchParams()
  if (query.day) params.set('day', query.day)
  if (query.folder && query.folder !== 'All') params.set('folder', query.folder)
  if (query.sourceId) params.set('sourceId', query.sourceId)
  const suffix = params.size ? `?${params}` : ''
  return request(`/v1/feed${suffix}`)
}

export function later(): Promise<RowModel[]> {
  return request<{ items: RowModel[] }>('/v1/later').then((body) => body.items)
}

export function read(q = '', limit = 80): Promise<RowModel[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (q.trim()) params.set('q', q.trim())
  return request<{ items: RowModel[] }>(`/v1/read?${params}`).then((body) => body.items)
}

export interface ItemDetail {
  item: Item
  source: Source
  state: UserItemState
  highlights: Highlight[]
}

export function item(id: string): Promise<ItemDetail> {
  return request(`/v1/items/${id}`)
}

export function patchState(
  id: string,
  patch: Partial<Pick<UserItemState, 'inLater' | 'readAt' | 'readProgress'>>,
): Promise<UserItemState> {
  return request(`/v1/items/${id}/state`, { method: 'PATCH', body: JSON.stringify(patch) })
}

export function createHighlight(itemId: string, highlight: Omit<Highlight, 'createdAt' | 'updatedAt'>): Promise<Highlight> {
  return request(`/v1/items/${itemId}/highlights`, { method: 'POST', body: JSON.stringify(highlight) })
}

export function patchHighlight(id: string, note: string): Promise<Highlight> {
  return request(`/v1/highlights/${id}`, { method: 'PATCH', body: JSON.stringify({ note }) })
}

export function deleteHighlight(id: string): Promise<void> {
  return request(`/v1/highlights/${id}`, { method: 'DELETE' })
}

export function sources(): Promise<Source[]> {
  return request<{ sources: Source[] }>('/v1/sources').then((body) => body.sources)
}

export function patchSource(id: string, patch: Record<string, unknown>): Promise<Source> {
  return request(`/v1/sources/${id}`, { method: 'PATCH', body: JSON.stringify(patch) })
}

export function createSource(body: Record<string, unknown>): Promise<Source> {
  return request('/v1/sources', { method: 'POST', body: JSON.stringify(body) })
}

export function deleteSource(id: string): Promise<void> {
  return request(`/v1/sources/${id}`, { method: 'DELETE' })
}

export function syncSource(id: string): Promise<{ synced: number; notModified: number; failed: number; itemsAdded: number }> {
  return request(`/v1/sources/${id}/sync`, { method: 'POST' })
}

export function syncAll(): Promise<{ synced: number; notModified: number; failed: number; itemsAdded: number }> {
  return request('/v1/sync', { method: 'POST' })
}

export function importSources(file: File): Promise<{ added: number; skipped: number; total: number }> {
  const body = new FormData()
  body.append('file', file)
  return request('/v1/sources/import', { method: 'POST', body })
}

export function ingest(body: Record<string, unknown> | { items: Record<string, unknown>[] }): Promise<{ items: Item[] }> {
  return request('/v1/ingest', { method: 'POST', body: JSON.stringify(body) })
}

export function preferences(): Promise<UserPreferences> {
  return request('/v1/preferences')
}

export function patchPreferences(patch: Partial<UserPreferences>): Promise<UserPreferences> {
  return request('/v1/preferences', { method: 'PATCH', body: JSON.stringify(patch) })
}
