import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { feed } from '../api.ts'
import { DayNav } from '../components/DayNav.tsx'
import { ItemList } from '../components/ItemList.tsx'
import { isDayKey, openAction } from '../lib/format.ts'
import { feedFilterNames } from '../lib/folders.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { useActions, useLibrary, useRefreshSources } from '../lib/hooks.ts'
import type { FeedResponse, RowModel } from '../types.ts'

function asFilter(value: string | null): string {
  if (!value) return 'All'
  const trimmed = value.trim()
  if (!trimmed || trimmed === 'All' || trimmed.length > 48) return 'All'
  if (!/^[\p{L}\p{N}][\p{L}\p{N} .&+'/-]*$/u.test(trimmed)) return 'All'
  return trimmed
}

export function FeedView() {
  const { sources, stateFor, rememberStates } = useLibrary()
  const { markRead, saveLater, removeLater } = useActions()
  const navigate = useNavigate()
  const { day: dayParam } = useParams()
  const [params, setParams] = useSearchParams()
  const filter = asFilter(params.get('folder'))
  const sourceId = params.get('source')
  const query = params.toString()
  const [data, setData] = useState<FeedResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [cursor, setCursor] = useState<string | null>(null)

  useEffect(() => {
    let stop = false
    setLoading(true)
    setError(null)
    feed({
      day: isDayKey(dayParam) ? dayParam : undefined,
      folder: filter,
      sourceId: sourceId ?? undefined,
    })
      .then((next) => {
        if (stop) return
        setData(next)
        setLoading(false)
        if (next.day && next.day !== dayParam) {
          navigate(query ? `/feed/${next.day}?${query}` : `/feed/${next.day}`, { replace: true })
        }
      })
      .catch((reason: unknown) => {
        if (stop) return
        setLoading(false)
        setError(reason instanceof Error ? reason.message : 'Could not load')
      })
    return () => {
      stop = true
    }
  }, [dayParam, filter, sourceId, navigate, query])

  const rows = (data?.items ?? []).map((row) => ({ ...row, state: stateFor(row.item.id, row.state) }))
  useRefreshSources(data?.items)
  useEffect(() => {
    if (data) rememberStates(data.items.map((row) => [row.item.id, row.state]))
  }, [data, rememberStates])

  const activeSource = sourceId ? sources.find((source) => source.id === sourceId) : undefined
  const day = data?.day || (isDayKey(dayParam) ? dayParam : '')

  const goDay = (nextDay: string) => {
    const query = params.toString()
    navigate(query ? `/feed/${nextDay}?${query}` : `/feed/${nextDay}`)
  }

  const setFilter = (name: string) => {
    const next = new URLSearchParams(params)
    next.delete('source')
    if (name === 'All') next.delete('folder')
    else next.set('folder', name)
    setParams(next)
  }

  const move = (delta: number) => {
    if (!rows.length) return
    const index = rows.findIndex((row) => row.item.id === cursor)
    const next = index < 0 ? (delta > 0 ? 0 : rows.length - 1) : Math.min(rows.length - 1, Math.max(0, index + delta))
    setCursor(rows[next]?.item.id ?? null)
  }

  const openRow = (row: RowModel) => {
    if (row.item.openMode === 'reader') navigate(`/item/${row.item.id}`, { state: { from: 'feed' } })
    else openAction(row.item.actionUrl)
  }

  const toggleLater = (id: string) => {
    if (stateFor(id, rows.find((row) => row.item.id === id)?.state ?? { inLater: false, savedAt: null, readAt: null, readProgress: 0 }).inLater) {
      removeLater(id)
    } else saveLater(id)
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
      const row = rows.find((entry) => entry.item.id === cursor)
      if (row) openAction(row.item.actionUrl)
    },
    Enter: () => {
      const row = rows.find((entry) => entry.item.id === cursor)
      if (row) openRow(row)
    },
  })

  const filters = feedFilterNames(sources)
  const shown = filter !== 'All' && !filters.includes(filter) ? [filters[0] ?? 'All', filter, ...filters.slice(1)] : filters
  const navReady = Boolean(data && data.day === day)

  return (
    <main className="sheet">
      <div className="filters">
        <div className="filter-names">
          {shown.map((name) => (
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
      {loading && !data ? <p className="meta">Loading</p> : null}
      {error ? <p className="meta">{error}</p> : null}
      <ItemList
        rows={rows}
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
          prevDay={navReady ? data?.prevDay : null}
          nextDay={navReady ? data?.nextDay : null}
          counts={navReady ? (data?.counts ?? {}) : {}}
          onDay={goDay}
        />
      ) : null}
    </main>
  )
}
