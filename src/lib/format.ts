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

export function normalizeUrl(input: string): string | null {
  const trimmed = input.trim()
  if (!trimmed) return null
  const withProto = /^[a-z][a-z0-9+.-]*:/i.test(trimmed) ? trimmed : `https://${trimmed}`
  try {
    const url = new URL(withProto)
    if (url.protocol.startsWith('http') && !url.hostname) return null
    return url.toString()
  } catch {
    return null
  }
}

export function isHttpUrl(url: string): boolean {
  return /^https?:/i.test(url)
}

export function openAction(url: string) {
  if (isHttpUrl(url)) {
    window.open(url, '_blank', 'noopener,noreferrer')
    return
  }
  window.location.href = url
}

export function auxiliaryParts(item: Item, extras: string[]): string[] {
  const parts = [...extras.filter(Boolean)]
  if (item.metadata.kind === 'hn') {
    parts.push(`${item.metadata.points} pts`, `${item.metadata.commentCount} comments`)
  }
  if (item.metadata.kind === 'youtube' || item.metadata.kind === 'podcast') {
    parts.push(`${item.metadata.durationMin} min`)
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
