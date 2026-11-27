import { useContext, useEffect, useRef } from 'react'
import type { Source } from '../types.ts'
import { LibraryContext, type Actions, type LibraryValue } from './context.ts'

export function useLibrary(): LibraryValue {
  const value = useContext(LibraryContext)
  if (!value) throw new Error('Store missing')
  return value
}

export function useActions(): Actions {
  return useLibrary().actions
}

function missingSourceKey(sources: Source[], rows: Array<{ source: Source }>): string {
  const ids = new Set(sources.map((source) => source.id))
  const folders = new Set<string>()
  for (const source of sources) {
    const folder = source.folder.trim()
    if (folder) folders.add(folder)
  }
  const missing = new Set<string>()
  for (const row of rows) {
    if (!ids.has(row.source.id)) missing.add(`id:${row.source.id}`)
    const folder = row.source.folder.trim()
    if (folder && !folders.has(folder)) missing.add(`folder:${folder}`)
  }
  return [...missing].sort().join('|')
}

export function useRefreshSources(rows: Array<{ source: Source }> | undefined): void {
  const { sources, ready } = useLibrary()
  const { reloadSources } = useActions()
  const refreshed = useRef('')

  useEffect(() => {
    if (!ready || !rows?.length) return
    const key = missingSourceKey(sources, rows)
    if (!key) {
      refreshed.current = ''
      return
    }
    if (refreshed.current === key) return
    refreshed.current = key
    void reloadSources().catch(() => {
      if (refreshed.current === key) refreshed.current = ''
    })
  }, [ready, rows, sources, reloadSources])
}
