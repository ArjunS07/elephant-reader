import { useEffect, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useEscape } from '../lib/hotkeys.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { ColumnAlign, ColumnWidth, FontFamily, ThemeName } from '../types.ts'

const NAV = [
  { to: '/feed', label: 'Feed' },
  { to: '/later', label: 'Later' },
  { to: '/read', label: 'Read' },
  { to: '/sources', label: 'Sources' },
] as const

function ElephantMark() {
  return (
    <svg
      className="mark"
      viewBox="0 0 28 28"
      width="22"
      height="22"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.9"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M5 19V13.5C5 9.5 8 7 12.5 7H16.5C19.8 7 22 9.2 22 12.2V16.5C22 18.2 23 19 24.3 19C25 19 25.5 18.5 25.5 17.8" />
      <path d="M5 19H9V15.5H13.5V19H17.5V14.5" />
      <path d="M13 10.5C11.2 10.5 10.5 12 10.8 13.8C11.1 15 12.3 15.3 13.5 14.5" />
      <circle cx="18.2" cy="11.2" r="0.85" fill="currentColor" stroke="none" />
    </svg>
  )
}

function currentPath(pathname: string, from: unknown): string {
  if (pathname === '/feed' || pathname.startsWith('/feed/')) return '/feed'
  if (!pathname.startsWith('/item')) return pathname
  if (from === 'later') return '/later'
  if (from === 'read') return '/read'
  if (from === 'feed') return '/feed'
  return ''
}

export function Header() {
  const { pathname, state } = useLocation()
  const from = (state as { from?: string } | null)?.from
  const current = currentPath(pathname, from)
  const reader = pathname.startsWith('/item/')
  const [open, setOpen] = useState(false)
  const panel = useRef<HTMLDivElement>(null)

  useEffect(() => {
    document.documentElement.dataset.surface = reader ? 'reader' : 'list'
  }, [reader])

  useEffect(() => {
    setOpen(false)
  }, [pathname])

  useEscape(open, () => setOpen(false))

  useEffect(() => {
    if (!open) return
    const onPointer = (event: MouseEvent) => {
      if (!panel.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onPointer)
    return () => document.removeEventListener('mousedown', onPointer)
  }, [open])

  return (
    <header className="top">
      <div className="bar">
        <Link to="/feed" className="wordmark">
          <ElephantMark />
          elephant
        </Link>
        <nav aria-label="Sections">
          {NAV.map((item) => (
            <Link key={item.to} to={item.to} aria-current={current === item.to ? 'page' : undefined}>
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="settings-anchor" ref={panel}>
          <button
            type="button"
            className="aa"
            aria-label="Reading settings"
            aria-expanded={open}
            aria-haspopup="dialog"
            onClick={() => setOpen((value) => !value)}
          >
            Aa
          </button>
          {open ? <Settings reader={reader} /> : null}
        </div>
      </div>
    </header>
  )
}

function Settings({ reader }: { reader: boolean }) {
  const { prefs } = useLibrary()
  const { setPrefs } = useActions()
  return (
    <div className="popover" role="dialog" aria-label="Reading settings" data-overlay>
      <div className="setting">
        <span>Theme</span>
        <div className="choices">
          {(['light', 'warm', 'dark'] as ThemeName[]).map((theme) => (
            <button
              key={theme}
              type="button"
              aria-pressed={prefs.theme === theme}
              onClick={() => setPrefs({ theme })}
            >
              {theme === 'light' ? 'Light' : theme === 'warm' ? 'Warm' : 'Dark'}
            </button>
          ))}
        </div>
      </div>
      {reader ? (
      <>
      <div className="setting">
        <span>Font</span>
        <div className="choices">
          {(
            [
              ['inter', 'Inter'],
              ['lora', 'Lora'],
              ['newsreader', 'Newsreader'],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              aria-pressed={prefs.fontFamily === value}
              onClick={() => setPrefs({ fontFamily: value satisfies FontFamily })}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
      <label className="setting slider">
        <span>Size</span>
        <input
          type="range"
          min={15}
          max={20}
          step={1}
          value={prefs.fontSize}
          aria-valuetext={`${prefs.fontSize}`}
          onChange={(event) => setPrefs({ fontSize: Number(event.target.value) })}
        />
        <span className="slider-value">{prefs.fontSize}</span>
      </label>
      <label className="setting slider">
        <span>Leading</span>
        <input
          type="range"
          min={1.5}
          max={1.85}
          step={0.05}
          value={prefs.lineHeight}
          aria-valuetext={prefs.lineHeight.toFixed(2)}
          onChange={(event) => setPrefs({ lineHeight: Number(event.target.value) })}
        />
        <span className="slider-value">{prefs.lineHeight.toFixed(2)}</span>
      </label>
      </>
      ) : null}
      <div className="setting">
        <span>Align</span>
        <div className="choices">
          {(['left', 'center'] as ColumnAlign[]).map((align) => (
            <button
              key={align}
              type="button"
              aria-pressed={prefs.align === align}
              onClick={() => setPrefs({ align })}
            >
              {align === 'left' ? 'Left' : 'Center'}
            </button>
          ))}
        </div>
      </div>
      {reader ? (
      <div className="setting">
        <span>Width</span>
        <div className="choices">
          {(
            [
              ['cozy', 'Cozy', 'Cozy, 600px'],
              ['standard', 'Standard', 'Standard, 672px'],
              ['wide', 'Wide', 'Wide, 760px'],
            ] as const
          ).map(([value, label, title]) => (
            <button
              key={value}
              type="button"
              title={title}
              aria-pressed={prefs.columnWidth === value}
              onClick={() => setPrefs({ columnWidth: value satisfies ColumnWidth })}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
      ) : null}
    </div>
  )
}
