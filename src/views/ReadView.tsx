import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ItemList } from '../components/ItemList.tsx'
import { openAction } from '../lib/format.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { matchesQuery, rowsFor, type RowModel } from '../lib/select.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'

export function ReadView() {
  const library = useLibrary()
  const { dismiss, markRead, saveLater } = useActions()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [cursor, setCursor] = useState<string | null>(null)
  const rows = useMemo(
    () => rowsFor(library, 'read').filter((row) => matchesQuery(row.item, library.highlights, query)),
    [library, query],
  )

  const move = (delta: number) => {
    if (!rows.length) return
    const index = rows.findIndex((row) => row.item.id === cursor)
    const next = index < 0 ? (delta > 0 ? 0 : rows.length - 1) : Math.min(rows.length - 1, Math.max(0, index + delta))
    setCursor(rows[next]?.item.id ?? null)
  }

  const openRow = (row: RowModel) => {
    if (row.item.openMode === 'reader') navigate(`/item/${row.item.id}`, { state: { from: 'read' } })
    else openAction(row.item.actionUrl)
  }

  useHotkeys({
    j: () => move(1),
    k: () => move(-1),
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
      <ItemList
        rows={rows}
        from="read"
        actions={[]}
        activeId={cursor}
        date="read"
        onActive={setCursor}
        onLater={saveLater}
        onRead={markRead}
        onDismiss={dismiss}
      />
    </main>
  )
}
