import type { Item } from '../types.ts'
import { quotesOf, tractionOf } from './quotes.ts'

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}`
}

export function dayKey(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const month = String(d.getUTCMonth() + 1).padStart(2, '0')
  const day = String(d.getUTCDate()).padStart(2, '0')
  return `${d.getUTCFullYear()}-${month}-${day}`
}

export function isDayKey(value: string | undefined): value is string {
  return Boolean(value && /^\d{4}-\d{2}-\d{2}$/.test(value))
}

export function shiftDay(key: string, delta: number): string {
  const [year, month, day] = key.split('-').map(Number)
  return dayKey(new Date(Date.UTC(year, month - 1, day + delta)).toISOString())
}

export function formatDayHeading(key: string): string {
  const [year, month, day] = key.split('-').map(Number)
  const date = new Date(Date.UTC(year, month - 1, day))
  return `${WEEKDAYS[date.getUTCDay()]}, ${MONTHS[month - 1]} ${day}`
}

export function monthLabel(year: number, month: number): string {
  return `${MONTHS[month - 1]} ${year}`
}

export function hostname(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}

const ALLOWED_PROTOCOLS = new Set(['http:', 'https:', 'overcast:'])

export function safeUrl(input: string | null | undefined): string | null {
  if (input == null) return null
  const trimmed = input.trim()
  if (!trimmed || /[\u0000-\u001F\u007F\s]/.test(trimmed)) return null
  let url: URL
  try {
    url = new URL(trimmed)
  } catch {
    return null
  }
  const protocol = url.protocol.toLowerCase()
  if (!ALLOWED_PROTOCOLS.has(protocol)) return null
  if ((protocol === 'http:' || protocol === 'https:') && !url.hostname) return null
  return trimmed
}

export function normalizeUrl(input: string): string | null {
  const trimmed = input.trim()
  if (!trimmed) return null
  const withProto = /^[a-z][a-z0-9+.-]*:/i.test(trimmed) ? trimmed : `https://${trimmed}`
  return safeUrl(withProto)
}

export function isHttpUrl(url: string): boolean {
  const safe = safeUrl(url)
  if (!safe) return false
  const protocol = new URL(safe).protocol
  return protocol === 'http:' || protocol === 'https:'
}

export function openAction(url: string) {
  const safe = safeUrl(url)
  if (!safe) return
  if (isHttpUrl(safe)) {
    window.open(safe, '_blank', 'noopener,noreferrer')
    return
  }
  window.location.href = safe
}

export function durationLabel(minutes: number | null | undefined): string | null {
  if (minutes == null || !Number.isFinite(minutes) || minutes <= 0) return null
  return `${minutes} min`
}

export function relativeTime(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return ''
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''
  const delta = now - then
  if (delta < 45_000) return 'just now'
  const minutes = Math.floor(delta / 60_000)
  if (minutes < 60) return `${Math.max(1, minutes)}m ago`
  const hours = Math.floor(delta / 3_600_000)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(delta / 86_400_000)
  if (days < 14) return `${days}d ago`
  return formatDate(iso)
}

export function auxiliaryParts(item: Item, extras: string[]): string[] {
  const parts = [...extras.filter(Boolean)]
  if (item.metadata.kind === 'hn') {
    parts.push(`${item.metadata.points} pts`, `${item.metadata.commentCount} comments`)
  }
  if (item.metadata.kind === 'youtube' || item.metadata.kind === 'podcast') {
    const duration = durationLabel(item.metadata.durationMin)
    if (duration) parts.push(duration)
  }
  const traction = tractionOf(item)
  if (traction) parts.push(traction)
  else {
    const quotes = quotesOf(item).length
    if (quotes === 1) parts.push('1 quote')
    else if (quotes > 1) parts.push(`${quotes} quotes`)
  }
  return parts
}

export function readPercent(progress: number): string | null {
  if (progress <= 0.01) return null
  return `${Math.round(progress * 100)}%`
}
