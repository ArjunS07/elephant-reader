import { useEffect, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useEscape } from '../lib/hotkeys.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { ColumnAlign, ColumnWidth, FontFamily, ThemeName } from '../types/model.ts'

const NAV = [
  { to: '/feed', label: 'Feed' },
  { to: '/later', label: 'Later' },
  { to: '/read', label: 'Read' },
  { to: '/sources', label: 'Sources' },
] as const

function currentPath(pathname: string, from: unknown): string {
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
  const [open, setOpen] = useState(false)
  const panel = useRef<HTMLDivElement>(null)

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
          Field
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
          {open ? <Settings /> : null}
        </div>
      </div>
    </header>
  )
}

function Settings() {
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
      <div className="setting">
        <span>Width</span>
        <div className="choices">
          {(
            [
              ['narrow', '600', 'Narrow, 600px'],
              ['default', '672', 'Default, 672px'],
              ['wide', '760', 'Wide, 760px'],
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
    </div>
  )
}
