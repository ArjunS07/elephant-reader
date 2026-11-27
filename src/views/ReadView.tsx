import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { read } from '../api.ts'
import { ItemList } from '../components/ItemList.tsx'
import { openAction } from '../lib/format.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { RowModel } from '../types.ts'

export function ReadView() {
  const { stateFor, rememberStates } = useLibrary()
  const { markRead, saveLater, removeLater } = useActions()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [rows, setRows] = useState<RowModel[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [cursor, setCursor] = useState<string | null>(null)

  useEffect(() => {
    let stop = false
    setLoading(true)
    const timer = window.setTimeout(() => {
      read(query)
        .then((items) => {
          if (stop) return
          setRows(items)
          setError(null)
          setLoading(false)
        })
        .catch((reason: unknown) => {
          if (stop) return
          setLoading(false)
          setError(reason instanceof Error ? reason.message : 'Could not load')
        })
    }, 160)
    return () => {
      stop = true
      window.clearTimeout(timer)
    }
  }, [query])

  useEffect(() => {
    rememberStates(rows.map((row) => [row.item.id, row.state]))
  }, [rows, rememberStates])

  const visible = rows.map((row) => ({ ...row, state: stateFor(row.item.id, row.state) }))

  const move = (delta: number) => {
    if (!visible.length) return
    const index = visible.findIndex((row) => row.item.id === cursor)
    const next = index < 0 ? (delta > 0 ? 0 : visible.length - 1) : Math.min(visible.length - 1, Math.max(0, index + delta))
    setCursor(visible[next]?.item.id ?? null)
  }

  const openRow = (row: RowModel) => {
    if (row.item.openMode === 'reader') navigate(`/item/${row.item.id}`, { state: { from: 'read' } })
    else openAction(row.item.actionUrl)
  }

  useHotkeys({
    j: () => move(1),
    k: () => move(-1),
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
      <input
        className="search"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Search"
        aria-label="Search"
        autoCapitalize="off"
        autoCorrect="off"
        spellCheck={false}
      />
      {loading && !visible.length ? <p className="meta">Loading</p> : null}
      {error ? <p className="meta">{error}</p> : null}
      <ItemList
        rows={visible}
        from="read"
        activeId={cursor}
        date="read"
        onActive={setCursor}
        onLater={saveLater}
        onRemove={removeLater}
        onRead={markRead}
        onSource={(id) => navigate(`/feed?source=${id}`)}
      />
    </main>
  )
}
