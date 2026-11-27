import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ItemList } from '../components/ItemList.tsx'
import { openAction } from '../lib/format.ts'
import { useHotkeys } from '../lib/hotkeys.ts'
import { rowsFor, type RowModel } from '../lib/select.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import { FOLDERS, type FolderFilter } from '../types/model.ts'

function asFolder(value: string | null): FolderFilter {
  return FOLDERS.includes(value as FolderFilter) ? (value as FolderFilter) : 'All'
}

export function FeedView() {
  const library = useLibrary()
  const { markRead, saveLater, removeLater } = useActions()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const folder = asFolder(params.get('folder'))
  const rows = rowsFor(library, 'feed', folder)
  const [cursor, setCursor] = useState<string | null>(null)

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
      <div className="filters">
        <div className="filter-names">
          {FOLDERS.map((name) => (
            <button
              key={name}
              type="button"
              aria-current={folder === name ? 'true' : undefined}
              onClick={() => {
                if (name === 'All') setParams({})
                else setParams({ folder: name })
              }}
            >
              {name}
            </button>
          ))}
        </div>
      </div>
      <ItemList
        rows={rows}
        from="feed"
        activeId={cursor}
        date="published"
        onActive={setCursor}
        onLater={saveLater}
        onRemove={removeLater}
        onRead={markRead}
      />
    </main>
  )
}
