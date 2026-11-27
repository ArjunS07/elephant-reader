import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { byline } from '../lib/article.ts'
import { excerpt } from '../lib/excerpt.ts'
import { readPercent } from '../lib/format.ts'
import type { RowModel } from '../lib/select.ts'
import { IconDiscussion, IconExternal } from './Icons.tsx'

interface ItemListProps {
  rows: RowModel[]
  from: 'feed' | 'later' | 'read'
  activeId: string | null
  showProgress?: boolean
  date: 'published' | 'read'
  onActive: (id: string) => void
  onLater: (id: string) => void
  onRemove: (id: string) => void
  onRead: (id: string) => void
}

export function ItemList({
  rows,
  from,
  activeId,
  showProgress = false,
  date,
  onActive,
  onLater,
  onRemove,
  onRead,
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
        const stub = from === 'read' ? null : excerpt(row.item)
        const inLater = row.state.inLater
        const isRead = row.state.readAt != null
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
            {stub ? <span className="excerpt">{stub}</span> : null}
          </>
        )
        return (
          <li
            key={row.item.id}
            className="row"
            data-item-id={row.item.id}
            data-active={row.item.id === activeId ? 'true' : 'false'}
            data-read={isRead ? 'true' : 'false'}
            data-later={inLater ? 'true' : 'false'}
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
            {from !== 'read' ? (
              <div className="actions">
                {from === 'feed' ? (
                  <button
                    type="button"
                    className={inLater ? 'text-action' : 'text-action primary'}
                    aria-pressed={inLater}
                    title={inLater ? 'In Later' : 'Read later (s)'}
                    onClick={() => (inLater ? onRemove(row.item.id) : onLater(row.item.id))}
                  >
                    {inLater ? 'In Later' : 'Read later'}
                  </button>
                ) : null}
                {from === 'later' ? (
                  <button type="button" className="text-action" title="Mark read (r)" onClick={() => onRead(row.item.id)}>
                    Mark read
                  </button>
                ) : null}
                {from === 'later' ? (
                  <button type="button" className="text-action" title="Remove" onClick={() => onRemove(row.item.id)}>
                    Remove
                  </button>
                ) : null}
                {row.item.discussionUrl ? (
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
