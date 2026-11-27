import { safeUrl } from './format.ts'

export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function unescapeHtml(value: string): string {
  return value
    .replace(/&quot;/g, '"')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
}

function linkHtml(label: string, href: string): string {
  const safe = safeUrl(unescapeHtml(href.trim()))
  if (!safe) return label
  return `<a href="${escapeHtml(safe)}" target="_blank" rel="noopener noreferrer">${label}</a>`
}

function replaceLinks(escaped: string): string {
  let out = ''
  let i = 0
  while (i < escaped.length) {
    const start = escaped.indexOf('[', i)
    if (start < 0 || escaped.indexOf('](', start) < 0) {
      out += escaped.slice(i)
      break
    }
    const mid = escaped.indexOf('](', start)
    const label = escaped.slice(start + 1, mid)
    if (!label || label.includes('[') || label.includes(']') || label.includes('\n')) {
      out += escaped.slice(i, start + 1)
      i = start + 1
      continue
    }
    let depth = 1
    let j = mid + 2
    for (; j < escaped.length; j += 1) {
      const ch = escaped[j]
      if (ch === '(') depth += 1
      else if (ch === ')') {
        depth -= 1
        if (depth === 0) break
      } else if (ch === '\n') break
    }
    if (depth !== 0) {
      out += escaped.slice(i, start + 1)
      i = start + 1
      continue
    }
    const href = escaped.slice(mid + 2, j)
    out += escaped.slice(i, start) + linkHtml(label, href)
    i = j + 1
  }
  return out
}

function inline(raw: string): string {
  let text = escapeHtml(raw)
  const codes: string[] = []
  text = text.replace(/`([^`]+)`/g, (_match, code: string) => {
    codes.push(`<code>${code}</code>`)
    return `\uE000${codes.length - 1}\uE000`
  })
  text = replaceLinks(text)
  text = text.replace(/\uE000(\d+)\uE000/g, (_match, index: string) => codes[Number(index)] ?? '')
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  text = text.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  return text
}

function isBlockStart(line: string): boolean {
  return (
    line.startsWith('```') ||
    line.startsWith('### ') ||
    line.startsWith('## ') ||
    line.startsWith('> ') ||
    line.startsWith('- ')
  )
}

export function markdownToHtml(source: string): string {
  const lines = source.replace(/\r\n/g, '\n').split('\n')
  const html: string[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i] ?? ''
    if (line.trim() === '') {
      i += 1
      continue
    }
    if (line.startsWith('```')) {
      const buf: string[] = []
      i += 1
      while (i < lines.length && !(lines[i] ?? '').startsWith('```')) {
        buf.push(lines[i] ?? '')
        i += 1
      }
      if (i < lines.length) i += 1
      html.push(`<pre><code>${escapeHtml(buf.join('\n'))}</code></pre>`)
      continue
    }
    if (line.startsWith('### ')) {
      html.push(`<h3>${inline(line.slice(4))}</h3>`)
      i += 1
      continue
    }
    if (line.startsWith('## ')) {
      html.push(`<h2>${inline(line.slice(3))}</h2>`)
      i += 1
      continue
    }
    if (line.startsWith('> ')) {
      const buf: string[] = []
      while (i < lines.length && (lines[i] ?? '').startsWith('> ')) {
        buf.push((lines[i] ?? '').slice(2))
        i += 1
      }
      html.push(`<blockquote><p>${inline(buf.join(' '))}</p></blockquote>`)
      continue
    }
    if (line.startsWith('- ')) {
      const buf: string[] = []
      while (i < lines.length && (lines[i] ?? '').startsWith('- ')) {
        buf.push(`<li>${inline((lines[i] ?? '').slice(2))}</li>`)
        i += 1
      }
      html.push(`<ul>${buf.join('')}</ul>`)
      continue
    }
    const buf: string[] = [line]
    i += 1
    while (i < lines.length && (lines[i] ?? '').trim() !== '' && !isBlockStart(lines[i] ?? '')) {
      buf.push(lines[i] ?? '')
      i += 1
    }
    html.push(`<p>${inline(buf.join(' '))}</p>`)
  }
  return html.join('')
}
