import type { FolderFilter, Highlight, Item, Source, UserItemState } from '../types/model.ts'

export interface Library {
  sources: Source[]
  items: Item[]
  states: Record<string, UserItemState>
  highlights: Highlight[]
}

export interface RowModel {
  item: Item
  source: Source
  state: UserItemState
}

const EMPTY_STATE: UserItemState = {
  inLater: false,
  savedAt: null,
  readAt: null,
  readProgress: 0,
}

export function stateOf(states: Record<string, UserItemState>, id: string): UserItemState {
  return states[id] ?? EMPTY_STATE
}

export function rowsFor(library: Library, mode: 'feed' | 'later' | 'read', folder: FolderFilter = 'All'): RowModel[] {
  const sourceById = new Map(library.sources.map((source) => [source.id, source]))
  const rows: RowModel[] = []
  for (const item of library.items) {
    const source = sourceById.get(item.sourceId)
    if (!source) continue
    if (mode !== 'read' && !source.enabled) continue
    const state = stateOf(library.states, item.id)
    if (mode === 'feed') {
      if (folder !== 'All' && source.folder !== folder) continue
    } else if (mode === 'later') {
      if (!state.inLater || state.readAt) continue
    } else if (!state.readAt) {
      continue
    }
    rows.push({ item, source, state })
  }
  if (mode === 'read') {
    rows.sort((a, b) => (b.state.readAt ?? '').localeCompare(a.state.readAt ?? ''))
  } else {
    rows.sort((a, b) => b.item.publishedAt.localeCompare(a.item.publishedAt))
  }
  return rows
}

export function matchesQuery(item: Item, highlights: Highlight[], query: string): boolean {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  const chunks: string[] = [item.title, item.author, item.plainText, item.contentMarkdown ?? '']
  if (item.metadata.kind === 'paper') {
    chunks.push(
      item.metadata.abstract,
      item.metadata.authors.join(' '),
      item.metadata.affiliations.join(' '),
      item.metadata.arxivId,
      ...item.metadata.discussionQuotes.map((quote) => `${quote.speaker} ${quote.text}`),
    )
  }
  if (item.metadata.kind === 'podcast') chunks.push(item.metadata.showName)
  if (item.metadata.kind === 'youtube') chunks.push(item.metadata.channelName)
  for (const highlight of highlights) {
    if (highlight.itemId === item.id) chunks.push(highlight.quote, highlight.note)
  }
  return chunks.join('\n').toLowerCase().includes(needle)
}
