import { useEffect, useRef, useState } from 'react'
import { formatDayHeading, monthLabel, shiftDay } from '../lib/format.ts'

const WEEK = ['S', 'M', 'T', 'W', 'T', 'F', 'S']

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
  earliest,
  latest,
  marked,
  onDay,
}: {
  day: string
  earliest: string
  latest: string
  marked: Set<string>
  onDay: (day: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [year, month] = day.split('-').map(Number)
  const [cursor, setCursor] = useState({ year, month })
  const root = useRef<HTMLDivElement>(null)
  const prev = day > earliest ? shiftDay(day, -1) : null
  const next = day < latest ? shiftDay(day, 1) : null

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

  const cells = cellsFor(cursor.year, cursor.month)

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
        type="button"
        className="calendar-toggle"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((value) => !value)}
      >
        Calendar
      </button>
      {open ? (
        <div className="calendar" role="dialog" aria-label="Calendar" data-overlay>
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
              return (
                <button
                  key={key}
                  type="button"
                  data-has={marked.has(key) ? 'true' : 'false'}
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
