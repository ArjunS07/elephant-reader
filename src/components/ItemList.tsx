import { useEffect, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { auxiliaryParts, formatDate, isHttpUrl, readPercent, safeUrl } from '../lib/format.ts'
import { labLine } from '../lib/quotes.ts'
import type { RowModel } from '../types.ts'
import { IconDiscussion, IconExternal } from './Icons.tsx'

function ignoreRowActivate(event: ReactMouseEvent): boolean {
  const target = event.target
  if (target instanceof Element && target.closest('a, button')) return true
  const selection = window.getSelection()
  return Boolean(selection && !selection.isCollapsed && selection.toString().trim())
}

function isYoutubeRow(row: RowModel): boolean {
  return row.item.metadata.kind === 'youtube' || row.source.folder === 'Video'
}

function listExcerpt(row: RowModel, from: ItemListProps['from']): string | null {
  if (from === 'read' || isYoutubeRow(row)) return null
  const excerpt = (row.item.excerpt ?? '').trim().replace(/^abstract\s*:\s*/i, '').trim()
  if (!excerpt || excerpt === row.item.title.trim()) return null
  return excerpt
}

function TitleBody({ title, children }: { title: string; children?: ReactNode }) {
  if (!children) return title
  const trimmed = title.trim()
  const splitAt = trimmed.lastIndexOf(' ')
  const head = splitAt > 0 ? `${trimmed.slice(0, splitAt)} ` : ''
  const tail = splitAt > 0 ? trimmed.slice(splitAt + 1) : trimmed
  return (
    <>
      {head}
      <span className="title-keep">
        {tail}
        {'\u00A0'}
        {children}
      </span>
    </>
  )
}

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
  onSource?: (id: string) => void
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
  onSource,
}: ItemListProps) {
  const navigate = useNavigate()
  useEffect(() => {
    if (!activeId) return
    document.querySelector<HTMLElement>(`[data-item-id="${CSS.escape(activeId)}"]`)?.scrollIntoView({ block: 'nearest' })
  }, [activeId])

  return (
    <ul className="items">
      {rows.map((row) => {
        const progress = showProgress ? readPercent(row.state.readProgress) : null
        const dateLabel =
          from === 'feed'
            ? null
            : formatDate(date === 'read' ? (row.state.readAt ?? row.item.publishedAt) : row.item.publishedAt)
        const aux = auxiliaryParts(row.item, [dateLabel ?? '', progress ?? ''])
        const external = row.item.openMode !== 'reader'
        const lab = labLine(row.item)
        const youtube = isYoutubeRow(row)
        const channel = row.item.metadata.kind === 'youtube' ? row.item.metadata.channelName.trim() : ''
        const attribName = youtube ? channel || row.item.author || row.source.name : row.item.author
        const sourceName = !youtube && !lab && row.source.name !== row.item.author ? row.source.name : null
        const stub = listExcerpt(row, from)
        const inLater = row.state.inLater
        const isRead = row.state.readAt != null
        const action = safeUrl(row.item.actionUrl)
        const discussion = safeUrl(row.item.discussionUrl)
        const open = (children: ReactNode, className?: string) =>
          row.item.openMode === 'reader' ? (
            <Link to={`/item/${row.item.id}`} state={{ from }} className={className}>
              {children}
            </Link>
          ) : action ? (
            <a className={className} href={action} {...(isHttpUrl(action) ? { target: '_blank', rel: 'noopener noreferrer' } : {})}>
              {children}
            </a>
          ) : (
            <span className={className}>{children}</span>
          )
        const openItem = (newTab: boolean) => {
          if (row.item.openMode === 'reader') {
            if (newTab) window.open(`/item/${row.item.id}`, '_blank', 'noopener')
            else navigate(`/item/${row.item.id}`, { state: { from } })
            return
          }
          if (!action) return
          if (newTab || isHttpUrl(action)) {
            window.open(action, '_blank', 'noopener,noreferrer')
            return
          }
          window.location.href = action
        }
        return (
          <li
            key={row.item.id}
            className="row"
            data-item-id={row.item.id}
            data-active={row.item.id === activeId ? 'true' : 'false'}
            data-read={isRead ? 'true' : 'false'}
            data-later={inLater ? 'true' : 'false'}
            onMouseEnter={() => onActive(row.item.id)}
            onClick={(event) => {
              if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
              if (ignoreRowActivate(event)) return
              openItem(false)
            }}
            onAuxClick={(event) => {
              if (event.button !== 1 || ignoreRowActivate(event)) return
              event.preventDefault()
              openItem(true)
            }}
          >
            <div className="row-main">
              {open(
                <span className="title">
                  <TitleBody title={row.item.title}>
                    {external ? (
                      <span className="external" title={row.item.openMode === 'deeplink' ? 'Deeplink' : 'External'}>
                        <IconExternal />
                      </span>
                    ) : null}
                  </TitleBody>
                </span>,
              )}
              <span className="attrib">
                {attribName}
                {sourceName ? (
                  <>
                    {' · '}
                    {onSource ? (
                      <button type="button" className="source-link" onClick={() => onSource(row.source.id)}>
                        {sourceName}
                      </button>
                    ) : (
                      sourceName
                    )}
                  </>
                ) : null}
              </span>
              {lab ? <span className="lab">{lab}</span> : null}
              {aux.length ? <span className="aux">{aux.join(' · ')}</span> : null}
              {stub ? open(<span className="excerpt">{stub}</span>) : null}
            </div>
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
                {discussion ? (
                  <a
                    className="icon-btn"
                    href={discussion}
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
