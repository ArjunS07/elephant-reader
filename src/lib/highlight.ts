import type { Highlight } from '../types/model.ts'

interface IndexedText {
  node: Text
  start: number
}

function collectText(root: HTMLElement): { nodes: IndexedText[]; text: string } {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const parent = (node as Text).parentElement
      if (parent?.closest('.margin-note, script, style')) return NodeFilter.FILTER_REJECT
      return NodeFilter.FILTER_ACCEPT
    },
  })
  const nodes: IndexedText[] = []
  let text = ''
  let current = walker.nextNode()
  while (current) {
    const node = current as Text
    nodes.push({ node, start: text.length })
    text += node.data
    current = walker.nextNode()
  }
  return { nodes, text }
}

function findQuote(text: string, quote: string, prefix: string, suffix: string): { start: number; end: number } | null {
  if (!quote) return null
  const hits: number[] = []
  let from = 0
  while (from <= text.length) {
    const at = text.indexOf(quote, from)
    if (at < 0) break
    hits.push(at)
    from = at + 1
  }
  if (!hits.length) return null
  let best = hits[0] ?? 0
  let bestScore = -1
  for (const at of hits) {
    const pre = text.slice(Math.max(0, at - prefix.length), at)
    const suf = text.slice(at + quote.length, at + quote.length + suffix.length)
    let score = 0
    if (prefix && pre.endsWith(prefix)) score += 3
    if (suffix && suf.startsWith(suffix)) score += 3
    if (score > bestScore) {
      bestScore = score
      best = at
    }
  }
  return { start: best, end: best + quote.length }
}

function rangeFromOffsets(nodes: IndexedText[], start: number, end: number): Range | null {
  let startNode: Text | null = null
  let startOffset = 0
  let endNode: Text | null = null
  let endOffset = 0
  for (const entry of nodes) {
    const nodeEnd = entry.start + entry.node.data.length
    if (!startNode && start >= entry.start && start <= nodeEnd) {
      startNode = entry.node
      startOffset = start - entry.start
    }
    if (!endNode && end >= entry.start && end <= nodeEnd) {
      endNode = entry.node
      endOffset = end - entry.start
    }
  }
  if (!startNode || !endNode) return null
  const range = document.createRange()
  range.setStart(startNode, startOffset)
  range.setEnd(endNode, endOffset)
  return range
}

function wrap(range: Range, id: string): HTMLElement {
  const mark = document.createElement('mark')
  mark.className = 'quote-mark'
  mark.dataset.hid = id
  try {
    range.surroundContents(mark)
  } catch {
    const contents = range.extractContents()
    mark.appendChild(contents)
    range.insertNode(mark)
  }
  return mark
}

export function layoutNotes(root: HTMLElement) {
  const notes = [...root.querySelectorAll<HTMLElement>('.margin-note')]
  const wide = window.matchMedia('(min-width: 1120px)').matches
  if (!wide) {
    for (const note of notes) note.style.top = ''
    return
  }
  const rootTop = root.getBoundingClientRect().top
  const boxes = notes
    .map((note) => {
      const mark = root.querySelector<HTMLElement>(`mark[data-hid="${CSS.escape(note.dataset.hid ?? '')}"]`)
      if (!mark) return null
      return { note, top: mark.getBoundingClientRect().top - rootTop }
    })
    .filter((box): box is { note: HTMLElement; top: number } => box !== null)
    .sort((a, b) => a.top - b.top)

  let lastBottom = -Infinity
  for (const box of boxes) {
    let top = box.top
    if (top < lastBottom + 10) top = lastBottom + 10
    box.note.style.top = `${top}px`
    lastBottom = top + box.note.offsetHeight
  }
}

export function applyHighlights(root: HTMLElement, highlights: Highlight[]) {
  const located: { highlight: Highlight; start: number; end: number }[] = []
  for (const highlight of highlights) {
    const { text } = collectText(root)
    const span = findQuote(text, highlight.quote, highlight.prefix, highlight.suffix)
    if (span) located.push({ highlight, ...span })
  }
  located.sort((a, b) => b.start - a.start)
  for (const entry of located) {
    const { nodes } = collectText(root)
    const range = rangeFromOffsets(nodes, entry.start, entry.end)
    if (!range) continue
    const mark = wrap(range, entry.highlight.id)
    const noteText = entry.highlight.note.trim()
    if (!noteText) continue
    const note = document.createElement('span')
    note.className = 'margin-note'
    note.dataset.hid = entry.highlight.id
    note.textContent = noteText
    const block = mark.closest('p, li, blockquote, h1, h2, h3')
    if (block) block.appendChild(note)
    else mark.insertAdjacentElement('afterend', note)
  }
  layoutNotes(root)
}

export function anchorFromRoot(root: HTMLElement, range: Range): { quote: string; prefix: string; suffix: string } | null {
  const { nodes, text } = collectText(root)
  const start = offsetAt(nodes, range.startContainer, range.startOffset)
  const end = offsetAt(nodes, range.endContainer, range.endOffset)
  if (start == null || end == null || end <= start) return null
  const quote = text.slice(start, end)
  if (!quote.trim()) return null
  return {
    quote,
    prefix: text.slice(Math.max(0, start - 40), start),
    suffix: text.slice(end, end + 40),
  }
}

function offsetAt(nodes: IndexedText[], container: Node, offset: number): number | null {
  if (container.nodeType === Node.TEXT_NODE) {
    const entry = nodes.find((node) => node.node === container)
    if (!entry) return null
    return entry.start + offset
  }
  if (container.nodeType === Node.ELEMENT_NODE) {
    const element = container as HTMLElement
    const child = element.childNodes[offset] ?? null
    if (!child) {
      const last = nodes.filter((node) => element.contains(node.node)).at(-1)
      return last ? last.start + last.node.data.length : null
    }
    if (child.nodeType === Node.TEXT_NODE) {
      const entry = nodes.find((node) => node.node === child)
      return entry ? entry.start : null
    }
    const first = nodes.find((node) => child.contains(node.node) || node.node === child)
    return first ? first.start : null
  }
  return null
}
