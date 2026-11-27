export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  return `${d.getDate()} ${months[d.getMonth()]}`
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

export function readPercent(progress: number): string | null {
  if (progress <= 0.01) return null
  return `${Math.round(progress * 100)}%`
}

export function makeAlias(name: string, taken: string[]): string {
  const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'letter'
  let alias = `nl-${slug}@read.local`
  let n = 2
  while (taken.includes(alias)) {
    alias = `nl-${slug}-${n}@read.local`
    n += 1
  }
  return alias
}
