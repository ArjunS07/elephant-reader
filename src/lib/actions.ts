import type { Highlight, Item, UserItemState, UserPreferences } from '../types/model.ts'
import { sanitizePrefs } from './storage.ts'
import { createSeed } from '../data/seed.ts'
import { hostname, makeAlias, normalizeUrl } from './format.ts'
import type { Library } from './select.ts'

export interface DataState extends Library {
  prefs: UserPreferences
}

export type DataAction =
  | { type: 'dismiss'; id: string; now: string }
  | { type: 'read'; id: string; now: string }
  | { type: 'later'; id: string; now: string }
  | { type: 'progress'; id: string; progress: number }
  | { type: 'toggle-source'; id: string }
  | { type: 'alias'; id: string }
  | { type: 'capture'; url: string; title: string; mode: 'later' | 'read'; now: string; id: string }
  | { type: 'add-highlight'; highlight: Highlight }
  | { type: 'patch-highlight'; id: string; note: string; now: string }
  | { type: 'delete-highlight'; id: string }
  | { type: 'prefs'; patch: Partial<UserPreferences> }
  | { type: 'reset' }

function withState(state: DataState, id: string, next: UserItemState): DataState {
  return { ...state, states: { ...state.states, [id]: next } }
}

export function reducer(state: DataState, action: DataAction): DataState {
  switch (action.type) {
    case 'dismiss': {
      const prev = state.states[action.id]
      if (!prev) return state
      return withState(state, action.id, {
        ...prev,
        queue: 'dismissed',
        dismissedAt: action.now,
      })
    }
    case 'read': {
      const prev = state.states[action.id]
      if (!prev) return state
      return withState(state, action.id, { ...prev, readAt: action.now })
    }
    case 'later': {
      const prev = state.states[action.id]
      if (!prev) return state
      return withState(state, action.id, {
        ...prev,
        queue: 'later',
        readAt: null,
        dismissedAt: null,
      })
    }
    case 'progress': {
      const prev = state.states[action.id]
      if (!prev) return state
      const progress = Math.min(1, Math.max(0, action.progress))
      if (Math.abs(prev.readProgress - progress) < 0.004) return state
      return withState(state, action.id, { ...prev, readProgress: progress })
    }
    case 'toggle-source':
      return {
        ...state,
        sources: state.sources.map((source) =>
          source.id === action.id ? { ...source, enabled: !source.enabled } : source,
        ),
      }
    case 'alias': {
      const source = state.sources.find((entry) => entry.id === action.id)
      if (!source || source.sourceType !== 'newsletter') return state
      const taken = state.sources.flatMap((entry) => (entry.emailAlias ? [entry.emailAlias] : []))
      const alias = makeAlias(source.name, taken)
      return {
        ...state,
        sources: state.sources.map((entry) => (entry.id === source.id ? { ...entry, emailAlias: alias } : entry)),
      }
    }
    case 'capture': {
      const source = state.sources.find((entry) => entry.sourceType === 'chrome_ext')
      if (!source) return state
      const url = normalizeUrl(action.url)
      if (!url) return state
      const title = action.title.trim() || hostname(url)
      const existing = state.items.find((item) => item.sourceId === source.id && item.canonicalUrl === url)
      const nextState: UserItemState =
        action.mode === 'later'
          ? {
              queue: 'later',
              readAt: null,
              readProgress: existing ? (state.states[existing.id]?.readProgress ?? 0) : 0,
              dismissedAt: null,
            }
          : { queue: 'none', readAt: action.now, readProgress: 1, dismissedAt: null }
      if (existing) {
        return {
          ...state,
          items: state.items.map((item) => (item.id === existing.id ? { ...item, title } : item)),
          states: { ...state.states, [existing.id]: nextState },
        }
      }
      const item: Item = {
        id: action.id,
        sourceId: source.id,
        externalId: url,
        canonicalUrl: url,
        title,
        author: hostname(url),
        publishedAt: action.now,
        openMode: 'external',
        actionUrl: url,
        plainText: '',
        metadata: { kind: 'generic' },
      }
      return {
        ...state,
        items: [item, ...state.items],
        states: { ...state.states, [item.id]: nextState },
      }
    }
    case 'add-highlight':
      if (state.highlights.some((highlight) => highlight.id === action.highlight.id)) return state
      return { ...state, highlights: [...state.highlights, action.highlight] }
    case 'patch-highlight': {
      const current = state.highlights.find((highlight) => highlight.id === action.id)
      if (!current || current.note === action.note) return state
      return {
        ...state,
        highlights: state.highlights.map((highlight) =>
          highlight.id === action.id ? { ...highlight, note: action.note, updatedAt: action.now } : highlight,
        ),
      }
    }
    case 'delete-highlight':
      return { ...state, highlights: state.highlights.filter((highlight) => highlight.id !== action.id) }
    case 'prefs': {
      const prefs = sanitizePrefs({ ...state.prefs, ...action.patch })
      return state.prefs === prefs ? state : { ...state, prefs }
    }
    case 'reset':
      return { ...createSeed(), prefs: state.prefs }
    default:
      return state
  }
}
