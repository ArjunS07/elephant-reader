import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { later } from '../api.ts'
import { ItemList } from '../components/ItemList.tsx'
import { openAction } from '../lib/format.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { RowModel } from '../types.ts'

export function LaterView() {
  const { stateFor, rememberStates } = useLibrary()
  const { markRead, saveLater, removeLater } = useActions()
  const navigate = useNavigate()
  const [loaded, setLoaded] = useState<RowModel[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [cursor, setCursor] = useState<string | null>(null)

  useEffect(() => {
    let stop = false
    later()
      .then((items) => {
        if (stop) return
        setLoaded(items)
        setLoading(false)
      })
      .catch((reason: unknown) => {
        if (stop) return
        setLoading(false)
        setError(reason instanceof Error ? reason.message : 'Could not load')
      })
    return () => {
      stop = true
    }
  }, [])

  useEffect(() => {
    rememberStates(loaded.map((row) => [row.item.id, row.state]))
  }, [loaded, rememberStates])

  const rows = loaded
    .map((row) => ({ ...row, state: stateFor(row.item.id, row.state) }))
    .filter((row) => row.state.inLater && !row.state.readAt)

  const move = (delta: number) => {
    if (!rows.length) return
    const index = rows.findIndex((row) => row.item.id === cursor)
    const next = index < 0 ? (delta > 0 ? 0 : rows.length - 1) : Math.min(rows.length - 1, Math.max(0, index + delta))
    setCursor(rows[next]?.item.id ?? null)
  }

  const openRow = (row: RowModel) => {
    if (row.item.openMode === 'reader') navigate(`/item/${row.item.id}`, { state: { from: 'later' } })
    else openAction(row.item.actionUrl)
  }

  useHotkeys({
    j: () => move(1),
    k: () => move(-1),
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

  return (
    <main className="sheet">
      {loading ? <p className="meta">Loading</p> : null}
      {error ? <p className="meta">{error}</p> : null}
      <ItemList
        rows={rows}
        from="later"
        activeId={cursor}
        showProgress
        date="published"
        onActive={setCursor}
        onLater={saveLater}
        onRemove={(id) => {
          const index = rows.findIndex((row) => row.item.id === id)
          removeLater(id)
          if (cursor === id) setCursor(rows[index + 1]?.item.id ?? rows[index - 1]?.item.id ?? null)
        }}
        onRead={markRead}
        onSource={(id) => navigate(`/feed?source=${id}`)}
      />
    </main>
  )
}
