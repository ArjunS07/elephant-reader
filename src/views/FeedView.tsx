import { useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { DayNav } from '../components/DayNav.tsx'
import { ItemList } from '../components/ItemList.tsx'
import { dayKey, isDayKey } from '../lib/format.ts'
import { openAction } from '../lib/format.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { rowsFor, type RowModel } from '../lib/select.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { Source } from '../types/model.ts'

const FILTERS = ['All', 'Papers', 'RSS', 'Newsletters', 'HN', 'Podcasts', 'Video', 'Briefings'] as const
type FeedFilter = (typeof FILTERS)[number]

function asFilter(value: string | null): FeedFilter {
  return FILTERS.includes(value as FeedFilter) ? (value as FeedFilter) : 'All'
}

function matchesFilter(source: Source, filter: FeedFilter, sourceId: string | null): boolean {
  if (sourceId) return source.id === sourceId
  if (filter === 'All') return true
  if (filter === 'RSS') return source.sourceType === 'rss'
  return source.folder === filter
}

export function FeedView() {
  const library = useLibrary()
  const { markRead, saveLater, removeLater } = useActions()
  const navigate = useNavigate()
  const { day: dayParam } = useParams()
  const [params, setParams] = useSearchParams()
  const filter = asFilter(params.get('folder'))
  const sourceId = params.get('source')
  const rows = rowsFor(library, 'feed')
  const scoped = useMemo(
    () => rows.filter((row) => matchesFilter(row.source, filter, sourceId)),
    [rows, filter, sourceId],
  )
  const days = useMemo(() => [...new Set(scoped.map((row) => dayKey(row.item.publishedAt)).filter(Boolean))].sort(), [scoped])
  const latest = days.at(-1) ?? ''
  const earliest = days[0] ?? latest
  const day = isDayKey(dayParam) ? dayParam : latest
  const visible = scoped.filter((row) => dayKey(row.item.publishedAt) === day)
  const [cursor, setCursor] = useState<string | null>(null)
  const activeSource = sourceId ? library.sources.find((source) => source.id === sourceId) : undefined

  const goDay = (nextDay: string) => {
    const query = params.toString()
    navigate(query ? `/feed/${nextDay}?${query}` : `/feed/${nextDay}`)
  }

  const setFilter = (name: FeedFilter) => {
    const next = new URLSearchParams(params)
    next.delete('source')
    if (name === 'All') next.delete('folder')
    else next.set('folder', name)
    setParams(next)
  }

  const move = (delta: number) => {
    if (!visible.length) return
    const index = visible.findIndex((row) => row.item.id === cursor)
    const next = index < 0 ? (delta > 0 ? 0 : visible.length - 1) : Math.min(visible.length - 1, Math.max(0, index + delta))
    setCursor(visible[next]?.item.id ?? null)
  }

  const openRow = (row: RowModel) => {
    if (row.item.openMode === 'reader') navigate(`/item/${row.item.id}`, { state: { from: 'feed' } })
    else openAction(row.item.actionUrl)
  }

  const toggleLater = (id: string) => {
    const state = library.states[id]
    if (state?.inLater) removeLater(id)
    else saveLater(id)
  }

  useHotkeys({
    j: () => move(1),
    k: () => move(-1),
    s: () => {
      if (cursor) toggleLater(cursor)
    },
    r: () => {
      if (cursor) markRead(cursor)
    },
    o: () => {
      const row = visible.find((entry) => entry.item.id === cursor)
      if (row) openAction(row.item.actionUrl)
    },
    Enter: () => {
      const row = visible.find((entry) => entry.item.id === cursor)
      if (row) openRow(row)
    },
  })

  return (
    <main className="sheet">
      <div className="filters">
        <div className="filter-names">
          {FILTERS.map((name) => (
            <button
              key={name}
              type="button"
              aria-current={!sourceId && filter === name ? 'true' : undefined}
              onClick={() => setFilter(name)}
            >
              {name}
            </button>
          ))}
          {activeSource ? (
            <button type="button" aria-current="true" onClick={() => setFilter('All')}>
              {activeSource.name}
            </button>
          ) : null}
        </div>
      </div>
      <ItemList
        rows={visible}
        from="feed"
        activeId={cursor}
        date="published"
        onActive={setCursor}
        onLater={saveLater}
        onRemove={removeLater}
        onRead={markRead}
        onSource={(id) => {
          const next = new URLSearchParams(params)
          next.delete('folder')
          next.set('source', id)
          setParams(next)
        }}
      />
      {day ? (
        <DayNav
          day={day}
          earliest={earliest || day}
          latest={latest || day}
          marked={new Set(days)}
          onDay={goDay}
        />
      ) : null}
    </main>
  )
}
