import { memo, useEffect, useLayoutEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { applyHighlights, anchorFromRoot, layoutNotes } from '../lib/highlight.ts'
import { useEscape } from '../lib/hotkeys.ts'
import { useActions } from '../lib/hooks.ts'
import type { Highlight, Item, OpenMode } from '../types.ts'
import { CommentaryList } from './CommentaryQuote.tsx'
import { quotesOf, tractionOf } from '../lib/quotes.ts'
import { IconExternal } from './Icons.tsx'
import { isHttpUrl, openAction, safeUrl } from '../lib/format.ts'

interface NavItem {
  id: string
  openMode: OpenMode
  actionUrl: string
}

interface ReaderProps {
  item: Item
  html: string
  hlKey: string
  highlights: Highlight[]
  initialProgress: number
  from: 'feed' | 'later' | 'read'
  navItems: NavItem[]
  inLater: boolean
  isRead: boolean
}

interface MenuState {
  quote: string
  prefix: string
  suffix: string
  top: number
  left: number
}

interface EditorState {
  id: string
  top: number
  left: number
}

function propsEqual(a: ReaderProps, b: ReaderProps): boolean {
  return (
    a.item.id === b.item.id &&
    a.html === b.html &&
    a.hlKey === b.hlKey &&
    a.initialProgress === b.initialProgress &&
    a.from === b.from &&
    a.inLater === b.inLater &&
    a.isRead === b.isRead &&
    a.navItems.map((item) => item.id).join('|') === b.navItems.map((item) => item.id).join('|')
  )
}

export const Reader = memo(function Reader({
  item,
  html,
  hlKey,
  highlights,
  initialProgress,
  from,
  navItems,
  inLater,
  isRead,
}: ReaderProps) {
  const ref = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const { markRead, saveLater, removeLater, setProgress, addHighlight, updateHighlight, deleteHighlight } = useActions()
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [editor, setEditor] = useState<EditorState | null>(null)
  const live = highlights.find((highlight) => highlight.id === editor?.id) ?? null

  const listPath = from === 'later' ? '/later' : from === 'read' ? '/read' : '/feed'
  const back = () => navigate(listPath)
  const original = safeUrl(item.actionUrl)

  useEscape(!editor && !menu, back)
  useEscape(Boolean(editor) || Boolean(menu), () => {
    setEditor(null)
    setMenu(null)
  })

  useLayoutEffect(() => {
    const root = ref.current
    if (!root) return
    root.innerHTML = html
    applyHighlights(root, highlights)
    const onLayout = () => layoutNotes(root)
    const frame = requestAnimationFrame(onLayout)
    const observer = new ResizeObserver(onLayout)
    observer.observe(root)
    const query = window.matchMedia('(min-width: 1120px)')
    query.addEventListener('change', onLayout)
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      query.removeEventListener('change', onLayout)
    }
  }, [html, hlKey, highlights])

  useEffect(() => {
    let armed = false
    let latest = initialProgress
    let timer = 0
    const arm = window.setTimeout(() => {
      if (latest > 0.02 && latest < 0.98) {
        const max = document.documentElement.scrollHeight - window.innerHeight
        if (max > 0) window.scrollTo(0, max * latest)
      } else {
        window.scrollTo(0, 0)
      }
      window.setTimeout(() => {
        armed = true
      }, 90)
    }, 40)
    const onScroll = () => {
      if (!armed) return
      const max = document.documentElement.scrollHeight - window.innerHeight
      latest = max <= 0 ? 0 : Math.min(1, window.scrollY / max)
      window.clearTimeout(timer)
      timer = window.setTimeout(() => setProgress(item.id, latest), 220)
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      window.clearTimeout(arm)
      window.clearTimeout(timer)
      window.removeEventListener('scroll', onScroll)
      if (armed) setProgress(item.id, latest)
    }
  }, [item.id, initialProgress, setProgress])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target
      if (target instanceof HTMLElement && target.closest('input, textarea, button, a, [contenteditable="true"]')) return
      if (document.querySelector('[data-overlay]')) return
      const index = navItems.findIndex((entry) => entry.id === item.id)
      const go = (delta: number) => {
        const next = navItems[index + delta]
        if (!next) return
        navigate(`/item/${next.id}`, { state: { from } })
      }
      if (event.key === 'j') {
        event.preventDefault()
        go(1)
      } else if (event.key === 'k') {
        event.preventDefault()
        go(-1)
      } else if (event.key === 's') {
        event.preventDefault()
        if (inLater) removeLater(item.id)
        else saveLater(item.id)
      } else if (event.key === 'r') {
        event.preventDefault()
        if (!isRead) {
          markRead(item.id)
          back()
        }
      } else if (event.key === 'o') {
        event.preventDefault()
        openAction(item.actionUrl)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  function onMouseUp() {
    const root = ref.current
    const selection = window.getSelection()
    if (!root || !selection || selection.isCollapsed || !selection.rangeCount) {
      setMenu(null)
      return
    }
    const range = selection.getRangeAt(0)
    if (!root.contains(range.commonAncestorContainer)) {
      setMenu(null)
      return
    }
    const anchor = anchorFromRoot(root, range)
    if (!anchor) {
      setMenu(null)
      return
    }
    const rect = range.getBoundingClientRect()
    setMenu({
      ...anchor,
      top: Math.max(8, rect.top),
      left: Math.min(window.innerWidth - 80, Math.max(80, rect.left + rect.width / 2)),
    })
  }

  function createHighlight(withNote: boolean) {
    if (!menu) return
    const highlight = addHighlight({
      itemId: item.id,
      quote: menu.quote,
      prefix: menu.prefix,
      suffix: menu.suffix,
      note: '',
    })
    const top = menu.top
    const left = menu.left
    setMenu(null)
    window.getSelection()?.removeAllRanges()
    if (withNote) setEditor({ id: highlight.id, top: top + 28, left: Math.max(8, left - 120) })
  }

  function openEditor(id: string, rect: DOMRect) {
    setMenu(null)
    setEditor({
      id,
      top: Math.min(window.innerHeight - 180, rect.bottom + 8),
      left: Math.min(window.innerWidth - 260, Math.max(8, rect.left)),
    })
  }

  function onClick(event: ReactMouseEvent) {
    const target = event.target
    if (!(target instanceof HTMLElement)) return
    const mark = target.closest<HTMLElement>('mark.quote-mark, .margin-note')
    if (!mark) return
    const selection = window.getSelection()
    if (selection && !selection.isCollapsed) return
    const id = mark.dataset.hid
    if (!id) return
    openEditor(id, mark.getBoundingClientRect())
  }

  useEffect(() => {
    if (!editor) return
    const node = document.getElementById('margin-note-input')
    if (node instanceof HTMLTextAreaElement) node.focus()
  }, [editor])

  return (
    <>
      <article className={item.metadata.kind === 'paper' ? 'reader paper' : 'reader'}>
        <h1 className="piece-title">
          <span>{item.title}</span>
          {original ? (
            <a
              className="open-original"
              href={original}
              title="Open original"
              aria-label="Open original"
              {...(isHttpUrl(original) ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
            >
              <IconExternal size={23} />
            </a>
          ) : null}
        </h1>
        <div
          ref={ref}
          className="reader-prose"
          onMouseUp={onMouseUp}
          onClick={onClick}
          dangerouslySetInnerHTML={{ __html: html }}
        />
        {tractionOf(item) ? <p className="traction-note">{tractionOf(item)}</p> : null}
        <CommentaryList quotes={quotesOf(item)} />
      </article>
      {menu ? (
        <div
          className="selection-menu"
          data-overlay
          style={{ top: menu.top, left: menu.left }}
          onMouseDown={(event) => event.preventDefault()}
        >
          <button type="button" onClick={() => createHighlight(false)}>
            Highlight
          </button>
          <button type="button" onClick={() => createHighlight(true)}>
            Note
          </button>
        </div>
      ) : null}
      {editor && live ? (
        <div
          className="note-editor"
          data-overlay
          data-note-editor
          style={{ top: editor.top, left: editor.left }}
        >
          <textarea
            id="margin-note-input"
            aria-label="Margin note"
            rows={3}
            value={live.note}
            onChange={(event) => updateHighlight(live.id, event.target.value)}
          />
          <div className="editor-actions">
            <button type="button" onClick={() => updateHighlight(live.id, '')}>
              Clear
            </button>
            <button
              type="button"
              onClick={() => {
                deleteHighlight(live.id)
                setEditor(null)
              }}
            >
              Remove
            </button>
          </div>
        </div>
      ) : null}
      <div className="actionbar">
        <button
          type="button"
          className={isRead ? 'text-action' : 'text-action primary'}
          aria-pressed={isRead}
          title={isRead ? 'Read' : 'Mark read (r)'}
          onClick={() => {
            if (isRead) return
            markRead(item.id)
            back()
          }}
        >
          {isRead ? 'Read' : 'Mark read'}
        </button>
      </div>
    </>
  )
}, propsEqual)
