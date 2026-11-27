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
  const { dismiss, dismissMany, markRead, saveLater } = useActions()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const folder = asFolder(params.get('folder'))
  const rows = rowsFor(library, 'feed', folder)
  const [cursor, setCursor] = useState<string | null>(null)

  const boundaryIndex = () => {
    if (cursor) {
      const index = rows.findIndex((row) => row.item.id === cursor)
      if (index >= 0) return index
    }
    const header = document.querySelector('header')
    const top = (header?.getBoundingClientRect().bottom ?? 0) + 1
    const nodes = document.querySelectorAll<HTMLElement>('[data-item-id]')
    for (const node of nodes) {
      if (node.getBoundingClientRect().bottom > top) {
        return rows.findIndex((row) => row.item.id === node.dataset.itemId)
      }
    }
    return 0
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

  useHotkeys({
    j: () => move(1),
    k: () => move(-1),
    s: () => {
      if (cursor) saveLater(cursor)
    },
    r: () => {
      if (cursor) markRead(cursor)
    },
    d: () => {
      if (!cursor) return
      const index = rows.findIndex((row) => row.item.id === cursor)
      dismiss(cursor)
      setCursor(rows[index + 1]?.item.id ?? rows[index - 1]?.item.id ?? null)
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
        {rows.length > 0 ? (
          <div className="sweep">
            <button
              type="button"
              title="Dismiss above"
              onClick={() => {
                const index = boundaryIndex()
                if (index <= 0) return
                dismissMany(rows.slice(0, index).map((row) => row.item.id))
              }}
            >
              Dismiss above
            </button>
            <button type="button" title="Dismiss all" onClick={() => dismissMany(rows.map((row) => row.item.id))}>
              Dismiss all
            </button>
          </div>
        ) : null}
      </div>
      <ItemList
        rows={rows}
        from="feed"
        actions={['later', 'read', 'dismiss', 'discussion']}
        activeId={cursor}
        date="published"
        onActive={setCursor}
        onLater={saveLater}
        onRead={markRead}
        onDismiss={(id) => {
          const index = rows.findIndex((row) => row.item.id === id)
          dismiss(id)
          if (cursor === id) setCursor(rows[index + 1]?.item.id ?? rows[index - 1]?.item.id ?? null)
        }}
      />
    </main>
  )
}
