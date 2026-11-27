import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ItemList } from '../components/ItemList.tsx'
import { openAction } from '../lib/format.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { rowsFor, type RowModel } from '../lib/select.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'

export function LaterView() {
  const library = useLibrary()
  const { markRead, saveLater, removeLater } = useActions()
  const navigate = useNavigate()
  const rows = rowsFor(library, 'later')
  const [cursor, setCursor] = useState<string | null>(null)

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
