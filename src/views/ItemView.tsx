import { useMemo } from 'react'
import { useLocation, useParams } from 'react-router-dom'
import { Reader } from '../components/Reader.tsx'
import { articleHtml } from '../lib/article.ts'
import { rowsFor } from '../lib/select.ts'
import { useLibrary } from '../lib/hooks.ts'

function originOf(from: unknown, queue: string, readAt: string | null): 'feed' | 'later' | 'read' {
  if (from === 'feed' || from === 'later' || from === 'read') return from
  if (readAt) return 'read'
  if (queue === 'later') return 'later'
  return 'feed'
}

export function ItemView() {
  const { id } = useParams()
  const library = useLibrary()
  const location = useLocation()
  const fromState = (location.state as { from?: string } | null)?.from
  const item = library.items.find((entry) => entry.id === id)
  const initialProgress = useMemo(() => {
    if (!item) return 0
    return library.states[item.id]?.readProgress ?? 0
    // Pin the progress from the moment this item opens. Later scroll updates must not retrigger the restore.
    // oxlint-disable-next-line react-hooks/exhaustive-deps
  }, [item?.id])

  if (!item) {
    return (
      <main className="sheet">
        <p className="meta missing">Not found</p>
      </main>
    )
  }

  const source = library.sources.find((entry) => entry.id === item.sourceId)
  const state = library.states[item.id] ?? {
    queue: 'firehose' as const,
    readAt: null,
    readProgress: 0,
    dismissedAt: null,
  }
  const from = originOf(fromState, state.queue, state.readAt)
  const highlights = library.highlights.filter((highlight) => highlight.itemId === item.id)
  const hlKey = highlights.map((highlight) => `${highlight.id}:${highlight.quote}:${highlight.note}`).join('|')
  const navItems = rowsFor(library, from).map((row) => ({
    id: row.item.id,
    openMode: row.item.openMode,
    actionUrl: row.item.actionUrl,
  }))

  return (
    <main className="reader-frame">
      <Reader
        key={item.id}
        item={item}
        html={articleHtml(item, source)}
        hlKey={hlKey}
        highlights={highlights}
        initialProgress={initialProgress}
        from={from}
        navItems={navItems}
      />
    </main>
  )
}
