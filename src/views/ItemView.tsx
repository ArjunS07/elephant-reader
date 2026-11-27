import { useEffect, useMemo, useState } from 'react'
import { useLocation, useParams } from 'react-router-dom'
import { ApiError, feed, item, later, read, type ItemDetail } from '../api.ts'
import { Reader } from '../components/Reader.tsx'
import { articleHtml } from '../lib/article.ts'
import { useLibrary } from '../lib/hooks.ts'
import type { OpenMode } from '../types.ts'

function originOf(from: unknown, inLater: boolean, readAt: string | null): 'feed' | 'later' | 'read' {
  if (from === 'feed' || from === 'later' || from === 'read') return from
  if (readAt) return 'read'
  if (inLater) return 'later'
  return 'feed'
}

export function ItemView() {
  const { id } = useParams()
  const { stateFor, rememberStates, highlightsFor, hydrateHighlights } = useLibrary()
  const location = useLocation()
  const fromState = (location.state as { from?: string } | null)?.from
  const [detail, setDetail] = useState<ItemDetail | null>(null)
  const [missing, setMissing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [navItems, setNavItems] = useState<Array<{ id: string; openMode: OpenMode; actionUrl: string }>>([])

  useEffect(() => {
    if (!id) return
    let stop = false
    setMissing(false)
    setError(null)
    item(id)
      .then((next) => {
        if (stop) return
        setDetail(next)
        rememberStates([[next.item.id, next.state]])
        hydrateHighlights(next.item.id, next.highlights)
        const from = originOf(fromState, next.state.inLater, next.state.readAt)
        const list =
          from === 'later' ? later() : from === 'read' ? read() : feed({ day: next.item.publishedDay }).then((body) => body.items)
        return list.then((rows) => {
          if (stop) return
          setNavItems(rows.map((row) => ({ id: row.item.id, openMode: row.item.openMode, actionUrl: row.item.actionUrl })))
        })
      })
      .catch((reason: unknown) => {
        if (stop) return
        if (reason instanceof ApiError && reason.status === 404) setMissing(true)
        else setError(reason instanceof Error ? reason.message : 'Could not load')
      })
    return () => {
      stop = true
    }
  }, [id, fromState, rememberStates, hydrateHighlights])

  const entry = detail && detail.item.id === id ? detail : null
  const state = entry ? stateFor(entry.item.id, entry.state) : null
  const highlights = entry ? (highlightsFor(entry.item.id) ?? entry.highlights) : []
  const pinnedId = entry?.item.id
  const initialProgress = useMemo(() => entry?.state.readProgress ?? 0, [pinnedId])

  if (missing) {
    return (
      <main className="sheet">
        <p className="meta missing">Not found</p>
      </main>
    )
  }
  if (error) {
    return (
      <main className="sheet">
        <p className="meta">{error}</p>
      </main>
    )
  }
  if (!entry || !state) {
    return (
      <main className="sheet">
        <p className="meta">Loading</p>
      </main>
    )
  }

  const from = originOf(fromState, state.inLater, state.readAt)
  const hlKey = highlights.map((highlight) => `${highlight.id}:${highlight.quote}:${highlight.note}`).join('|')

  return (
    <main className="reader-frame">
      <Reader
        key={entry.item.id}
        item={entry.item}
        html={articleHtml(entry.item, entry.source)}
        hlKey={hlKey}
        highlights={highlights}
        initialProgress={initialProgress}
        from={from}
        navItems={navItems}
        inLater={state.inLater}
        isRead={state.readAt != null}
      />
    </main>
  )
}
