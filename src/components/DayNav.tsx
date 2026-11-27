import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { formatDayHeading, monthLabel } from '../lib/format.ts'

const WEEK = ['S', 'M', 'T', 'W', 'T', 'F', 'S']

function volumeBucket(count: number, max: number): 0 | 1 | 2 | 3 {
  if (count <= 0 || max <= 0) return 0
  if (max === 1) return 1
  const ratio = count / max
  if (ratio < 0.4) return 1
  if (ratio < 0.75) return 2
  return 3
}

function cellsFor(year: number, month: number): Array<number | null> {
  const first = new Date(Date.UTC(year, month - 1, 1)).getUTCDay()
  const count = new Date(Date.UTC(year, month, 0)).getUTCDate()
  const cells: Array<number | null> = Array.from({ length: first }, () => null)
  for (let day = 1; day <= count; day += 1) cells.push(day)
  while (cells.length % 7 !== 0) cells.push(null)
  return cells
}

export function DayNav({
  day,
  prevDay,
  nextDay,
  counts,
  onDay,
}: {
  day: string
  prevDay?: string | null
  nextDay?: string | null
  counts: Record<string, number>
  onDay: (day: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [place, setPlace] = useState<{ top: number; left: number } | null>(null)
  const [year, month] = day.split('-').map(Number)
  const [cursor, setCursor] = useState({ year, month })
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const popover = useRef<HTMLDivElement>(null)
  const prev = prevDay?.trim() ? prevDay : null
  const next = nextDay?.trim() ? nextDay : null

  useEffect(() => {
    setCursor({ year, month })
  }, [year, month])

  useEffect(() => {
    if (!open) return
    const onPointer = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onPointer)
    return () => document.removeEventListener('mousedown', onPointer)
  }, [open])

  useLayoutEffect(() => {
    if (!open) {
      setPlace(null)
      return
    }
    const pad = 12
    const gap = 8
    const measure = () => {
      const button = trigger.current
      const panel = popover.current
      if (!button || !panel) return
      const rect = button.getBoundingClientRect()
      const width = panel.offsetWidth
      const height = panel.offsetHeight
      const spaceAbove = rect.top - pad
      const spaceBelow = window.innerHeight - rect.bottom - pad
      const openUp = spaceAbove >= height + gap || spaceAbove > spaceBelow
      let top = openUp ? rect.top - height - gap : rect.bottom + gap
      top = Math.max(pad, Math.min(top, window.innerHeight - height - pad))
      let left = rect.left
      left = Math.max(pad, Math.min(left, window.innerWidth - width - pad))
      setPlace((prev) => (prev && prev.top === top && prev.left === left ? prev : { top, left }))
    }
    measure()
    window.addEventListener('resize', measure)
    window.addEventListener('scroll', measure, true)
    return () => {
      window.removeEventListener('resize', measure)
      window.removeEventListener('scroll', measure, true)
    }
  }, [open, cursor])

  const cells = cellsFor(cursor.year, cursor.month)
  const maxCount = Math.max(0, ...Object.values(counts))

  return (
    <div className="daynav" ref={root}>
      <button type="button" className="caret" aria-label="Previous day" disabled={!prev} onClick={() => prev && onDay(prev)}>
        ‹
      </button>
      <span className="day-label">{formatDayHeading(day)}</span>
      <button type="button" className="caret" aria-label="Next day" disabled={!next} onClick={() => next && onDay(next)}>
        ›
      </button>
      <button
        ref={trigger}
        type="button"
        className="calendar-toggle"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((value) => !value)}
      >
        Calendar
      </button>
      {open ? (
        <div
          ref={popover}
          className="calendar"
          role="dialog"
          aria-label="Calendar"
          data-overlay
          style={{
            top: place?.top ?? 0,
            left: place?.left ?? 0,
            visibility: place ? 'visible' : 'hidden',
          }}
        >
          <div className="calendar-head">
            <button
              type="button"
              className="caret"
              aria-label="Previous month"
              onClick={() =>
                setCursor((value) =>
                  value.month === 1 ? { year: value.year - 1, month: 12 } : { year: value.year, month: value.month - 1 },
                )
              }
            >
              ‹
            </button>
            <span>{monthLabel(cursor.year, cursor.month)}</span>
            <button
              type="button"
              className="caret"
              aria-label="Next month"
              onClick={() =>
                setCursor((value) =>
                  value.month === 12 ? { year: value.year + 1, month: 1 } : { year: value.year, month: value.month + 1 },
                )
              }
            >
              ›
            </button>
          </div>
          <div className="calendar-grid">
            {WEEK.map((label, index) => (
              <span key={`${label}-${index}`} className="calendar-dow">
                {label}
              </span>
            ))}
            {cells.map((value, index) => {
              if (!value) return <span key={`empty-${index}`} />
              const key = `${cursor.year}-${String(cursor.month).padStart(2, '0')}-${String(value).padStart(2, '0')}`
              const count = counts[key] ?? 0
              return (
                <button
                  key={key}
                  type="button"
                  data-bucket={volumeBucket(count, maxCount)}
                  title={count === 1 ? '1 item' : `${count} items`}
                  aria-current={key === day ? 'date' : undefined}
                  onClick={() => {
                    onDay(key)
                    setOpen(false)
                  }}
                >
                  {value}
                </button>
              )
            })}
          </div>
        </div>
      ) : null}
    </div>
  )
}
