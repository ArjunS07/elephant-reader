import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { byline } from '../lib/article.ts'
import { readPercent } from '../lib/format.ts'
import type { RowModel } from '../lib/select.ts'
import { IconBookmark, IconCheck, IconClose, IconDiscussion, IconExternal } from './Icons.tsx'

export type RowAction = 'later' | 'read' | 'dismiss' | 'discussion'

interface ItemListProps {
  rows: RowModel[]
  from: 'feed' | 'later' | 'read'
  actions: RowAction[]
  activeId: string | null
  showProgress?: boolean
  date: 'published' | 'read'
  onActive: (id: string) => void
  onLater: (id: string) => void
  onRead: (id: string) => void
  onDismiss: (id: string) => void
}

export function ItemList({
  rows,
  from,
  actions,
  activeId,
  showProgress = false,
  date,
  onActive,
  onLater,
  onRead,
  onDismiss,
}: ItemListProps) {
  useEffect(() => {
    if (!activeId) return
    document.querySelector<HTMLElement>(`[data-item-id="${CSS.escape(activeId)}"]`)?.scrollIntoView({ block: 'nearest' })
  }, [activeId])

  return (
    <ul className="items">
      {rows.map((row) => {
        const progress = showProgress ? readPercent(row.state.readProgress) : null
        const meta = byline(row.item, row.source, date === 'read' ? (row.state.readAt ?? row.item.publishedAt) : row.item.publishedAt)
        const external = row.item.openMode !== 'reader'
        const title = (
          <>
            <span className="title">
              {row.item.title}
              {external ? (
                <span className="external" title={row.item.openMode === 'deeplink' ? 'Deeplink' : 'External'}>
                  <IconExternal />
                </span>
              ) : null}
            </span>
            <span className="meta">
              {meta}
              {progress ? (
                <span title={`${progress} read`}> · {progress}</span>
              ) : null}
            </span>
          </>
        )
        return (
          <li
            key={row.item.id}
            className="row"
            data-item-id={row.item.id}
            data-active={row.item.id === activeId ? 'true' : 'false'}
            onMouseEnter={() => onActive(row.item.id)}
          >
            {row.item.openMode === 'reader' ? (
              <Link to={`/item/${row.item.id}`} state={{ from }} className="row-main">
                {title}
              </Link>
            ) : (
              <a className="row-main" href={row.item.actionUrl} target="_blank" rel="noopener noreferrer">
                {title}
              </a>
            )}
            {actions.length > 0 ? (
              <div className="actions">
                {actions.includes('later') ? (
                  <button type="button" className="icon-btn" title="Later (s)" aria-label="Later" onClick={() => onLater(row.item.id)}>
                    <IconBookmark />
                  </button>
                ) : null}
                {actions.includes('read') ? (
                  <button type="button" className="icon-btn" title="Mark as read (r)" aria-label="Mark as read" onClick={() => onRead(row.item.id)}>
                    <IconCheck />
                  </button>
                ) : null}
                {actions.includes('dismiss') ? (
                  <button type="button" className="icon-btn" title="Dismiss (d)" aria-label="Dismiss" onClick={() => onDismiss(row.item.id)}>
                    <IconClose />
                  </button>
                ) : null}
                {actions.includes('discussion') && row.item.discussionUrl ? (
                  <a
                    className="icon-btn"
                    href={row.item.discussionUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    title="Discussion"
                    aria-label="Discussion"
                  >
                    <IconDiscussion />
                  </a>
                ) : null}
              </div>
            ) : null}
          </li>
        )
      })}
    </ul>
  )
}
