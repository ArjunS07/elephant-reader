import { useCallback, useEffect, useMemo, useReducer, type ReactNode } from 'react'
import type { Highlight } from '../types/model.ts'
import { reducer, type DataAction, type DataState } from './actions.ts'
import { ActionsContext, LibraryContext, type Actions } from './context.ts'
import { createSeed } from '../data/seed.ts'
import { normalizeUrl } from './format.ts'
import type { Library } from './select.ts'
import { applyPrefs, DATA_KEY, loadPrefs } from './storage.ts'

function init(): DataState {
  const prefs = loadPrefs()
  applyPrefs(prefs)
  return { ...loadLibrary(), prefs }
}

function loadLibrary(): Library {
  try {
    const raw = localStorage.getItem(DATA_KEY)
    if (!raw) return createSeed()
    const parsed = JSON.parse(raw) as Partial<Library>
    if (!Array.isArray(parsed.sources) || !Array.isArray(parsed.items) || !parsed.states || !Array.isArray(parsed.highlights)) {
      return createSeed()
    }
    return parsed as Library
  } catch {
    return createSeed()
  }
}

function persist(state: DataState) {
  try {
    const library: Library = {
      sources: state.sources,
      items: state.items,
      states: state.states,
      highlights: state.highlights,
    }
    localStorage.setItem(DATA_KEY, JSON.stringify(library))
  } catch {
    /* ignore quota */
  }
}

export function StoreProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, null, () => init())

  useEffect(() => {
    persist(state)
  }, [state])

  useEffect(() => {
    applyPrefs(state.prefs)
  }, [state.prefs])

  const send = useCallback((action: DataAction) => {
    dispatch(action)
  }, [])

  const actions = useMemo<Actions>(
    () => ({
      dismiss: (id) => send({ type: 'dismiss', id, now: new Date().toISOString() }),
      dismissMany: (ids) => {
        const now = new Date().toISOString()
        for (const id of ids) send({ type: 'dismiss', id, now })
      },
      markRead: (id) => send({ type: 'read', id, now: new Date().toISOString() }),
      saveLater: (id) => send({ type: 'later', id, now: new Date().toISOString() }),
      setProgress: (id, progress) => send({ type: 'progress', id, progress }),
      toggleSource: (id) => send({ type: 'toggle-source', id }),
      generateAlias: (id) => send({ type: 'alias', id }),
      capture: ({ url, title, mode }) => {
        if (!normalizeUrl(url)) return false
        const id = crypto.randomUUID()
        send({ type: 'capture', url, title, mode, now: new Date().toISOString(), id })
        return true
      },
      addHighlight: (input) => {
        const now = new Date().toISOString()
        const highlight: Highlight = { ...input, id: crypto.randomUUID(), createdAt: now, updatedAt: now }
        send({ type: 'add-highlight', highlight })
        return highlight
      },
      updateHighlight: (id, note) => send({ type: 'patch-highlight', id, note, now: new Date().toISOString() }),
      deleteHighlight: (id) => send({ type: 'delete-highlight', id }),
      setPrefs: (patch) => send({ type: 'prefs', patch }),
      resetDemo: () => {
        try {
          localStorage.removeItem(DATA_KEY)
        } catch {
          /* ignore */
        }
        send({ type: 'reset' })
      },
    }),
    [send],
  )

  return (
    <ActionsContext.Provider value={actions}>
      <LibraryContext.Provider value={state}>{children}</LibraryContext.Provider>
    </ActionsContext.Provider>
  )
}

