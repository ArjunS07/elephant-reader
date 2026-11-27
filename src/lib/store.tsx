import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import * as api from '../api.ts'
import type { Highlight, Source, UserItemState, UserPreferences } from '../types.ts'
import { LibraryContext, type Actions } from './context.ts'
import { hostname, normalizeUrl } from './format.ts'
import { applyPrefs, loadPrefs, sanitizePrefs } from './storage.ts'

const EMPTY: UserItemState = { inLater: false, savedAt: null, readAt: null, readProgress: 0 }

export function StoreProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefsState] = useState<UserPreferences>(() => loadPrefs())
  const [sources, setSources] = useState<Source[]>([])
  const [ready, setReady] = useState(false)
  const [states, setStates] = useState<Record<string, UserItemState>>({})
  const [highlights, setHighlights] = useState<Record<string, Highlight[]>>({})
  const known = useRef<Record<string, UserItemState>>({})
  const dirtyState = useRef(new Set<string>())
  const dirtyItems = useRef(new Set<string>())
  const noteTimers = useRef<Record<string, number>>({})

  const reloadSources = useCallback(async () => {
    setSources(await api.sources())
  }, [])

  useEffect(() => {
    let stop = false
    applyPrefs(loadPrefs())
    Promise.all([api.preferences(), api.sources()])
      .then(([nextPrefs, nextSources]) => {
        if (stop) return
        const clean = sanitizePrefs(nextPrefs)
        setPrefsState(clean)
        applyPrefs(clean)
        setSources(nextSources)
      })
      .catch(() => {
        /* keep the pre-paint hint when the API is still starting */
      })
      .finally(() => {
        if (!stop) setReady(true)
      })
    return () => {
      stop = true
    }
  }, [])

  const rememberStates = useCallback((entries: Array<[string, UserItemState]>) => {
    setStates((current) => {
      let changed = false
      const next = { ...current }
      for (const [id, state] of entries) {
        if (dirtyState.current.has(id)) continue
        known.current[id] = state
        if (next[id] !== state) {
          next[id] = state
          changed = true
        }
      }
      return changed ? next : current
    })
  }, [])

  const stateFor = useCallback(
    (id: string, fallback: UserItemState) => states[id] ?? known.current[id] ?? fallback,
    [states],
  )

  const hydrateHighlights = useCallback((id: string, list: Highlight[]) => {
    if (dirtyItems.current.has(id)) return
    setHighlights((current) => ({ ...current, [id]: list }))
  }, [])

  const highlightsFor = useCallback((id: string) => highlights[id], [highlights])

  const writeState = useCallback((id: string, next: UserItemState) => {
    dirtyState.current.add(id)
    known.current[id] = next
    setStates((current) => ({ ...current, [id]: next }))
  }, [])

  const actions = useMemo<Actions>(() => {
    const base = (id: string) => known.current[id] ?? EMPTY
    const settle = (id: string, prev: UserItemState, promise: Promise<UserItemState>) => {
      promise
        .then((server) => {
          dirtyState.current.delete(id)
          known.current[id] = server
          setStates((current) => ({ ...current, [id]: server }))
        })
        .catch(() => {
          dirtyState.current.delete(id)
          known.current[id] = prev
          setStates((current) => ({ ...current, [id]: prev }))
        })
    }
    return {
      markRead: (id) => {
        const prev = base(id)
        const readAt = new Date().toISOString()
        writeState(id, { ...prev, readAt })
        settle(id, prev, api.patchState(id, { readAt }))
      },
      saveLater: (id) => {
        const prev = base(id)
        const savedAt = prev.savedAt ?? new Date().toISOString()
        writeState(id, { ...prev, inLater: true, savedAt })
        settle(id, prev, api.patchState(id, { inLater: true }))
      },
      removeLater: (id) => {
        const prev = base(id)
        writeState(id, { ...prev, inLater: false, savedAt: null })
        settle(id, prev, api.patchState(id, { inLater: false }))
      },
      setProgress: (id, progress) => {
        const prev = base(id)
        const readProgress = Math.min(1, Math.max(0, progress))
        if (Math.abs(prev.readProgress - readProgress) < 0.004) return
        writeState(id, { ...prev, readProgress })
        settle(id, prev, api.patchState(id, { readProgress }))
      },
      toggleSource: async (id) => {
        const source = sources.find((entry) => entry.id === id)
        if (!source) return
        const next = await api.patchSource(id, { enabled: !source.enabled })
        setSources((list) => list.map((entry) => (entry.id === id ? next : entry)))
      },
      generateAlias: async (id) => {
        const next = await api.patchSource(id, { generateAlias: true })
        setSources((list) => list.map((entry) => (entry.id === id ? next : entry)))
      },
      capture: async ({ url, title, mode }) => {
        const canonical = normalizeUrl(url)
        if (!canonical) return false
        const now = new Date().toISOString()
        await api.ingest({
          canonicalUrl: canonical,
          title: title.trim() || hostname(canonical),
          author: hostname(canonical),
          publishedAt: now,
          openMode: 'external',
          actionUrl: canonical,
          plainText: '',
          initialState: mode === 'later' ? { inLater: true } : { readAt: now },
        })
        return true
      },
      addHighlight: (input) => {
        const now = new Date().toISOString()
        const highlight: Highlight = { ...input, id: crypto.randomUUID(), createdAt: now, updatedAt: now }
        dirtyItems.current.add(input.itemId)
        setHighlights((current) => ({
          ...current,
          [input.itemId]: [...(current[input.itemId] ?? []), highlight],
        }))
        api
          .createHighlight(input.itemId, highlight)
          .then((saved) => {
            setHighlights((current) => ({
              ...current,
              [input.itemId]: (current[input.itemId] ?? []).map((entry) => (entry.id === highlight.id ? saved : entry)),
            }))
          })
          .catch(() => {
            setHighlights((current) => ({
              ...current,
              [input.itemId]: (current[input.itemId] ?? []).filter((entry) => entry.id !== highlight.id),
            }))
          })
          .finally(() => {
            if (!noteTimers.current[highlight.id]) dirtyItems.current.delete(input.itemId)
          })
        return highlight
      },
      updateHighlight: (id, note) => {
        let itemId = ''
        setHighlights((current) => {
          const next = { ...current }
          for (const key of Object.keys(next)) {
            if (!next[key].some((entry) => entry.id === id)) continue
            itemId = key
            dirtyItems.current.add(key)
            next[key] = next[key].map((entry) =>
              entry.id === id ? { ...entry, note, updatedAt: new Date().toISOString() } : entry,
            )
          }
          return next
        })
        window.clearTimeout(noteTimers.current[id])
        noteTimers.current[id] = window.setTimeout(() => {
          delete noteTimers.current[id]
          api
            .patchHighlight(id, note)
            .catch(() => undefined)
            .finally(() => {
              if (itemId) dirtyItems.current.delete(itemId)
            })
        }, 280)
      },
      deleteHighlight: (id) => {
        let itemId = ''
        let removed: Highlight | undefined
        setHighlights((current) => {
          const next = { ...current }
          for (const key of Object.keys(next)) {
            const found = next[key].find((entry) => entry.id === id)
            if (!found) continue
            itemId = key
            removed = found
            dirtyItems.current.add(key)
            next[key] = next[key].filter((entry) => entry.id !== id)
          }
          return next
        })
        window.clearTimeout(noteTimers.current[id])
        delete noteTimers.current[id]
        api.deleteHighlight(id).catch(() => {
          if (!itemId || !removed) return
          const highlight = removed
          setHighlights((current) => ({ ...current, [itemId]: [...(current[itemId] ?? []), highlight] }))
        }).finally(() => {
          if (itemId) dirtyItems.current.delete(itemId)
        })
      },
      setPrefs: (patch) => {
        const prev = prefs
        const next = sanitizePrefs({ ...prev, ...patch })
        setPrefsState(next)
        applyPrefs(next)
        api.patchPreferences(patch).then((saved) => {
          const clean = sanitizePrefs(saved)
          setPrefsState(clean)
          applyPrefs(clean)
        }).catch(() => {
          setPrefsState(prev)
          applyPrefs(prev)
        })
      },
      reloadSources,
    }
  }, [prefs, reloadSources, sources, writeState])

  const value = useMemo(
    () => ({
      prefs,
      sources,
      ready,
      stateFor,
      rememberStates,
      highlightsFor,
      hydrateHighlights,
      actions,
    }),
    [prefs, sources, ready, stateFor, rememberStates, highlightsFor, hydrateHighlights, actions],
  )

  return <LibraryContext.Provider value={value}>{children}</LibraryContext.Provider>
}
