import { createContext } from 'react'
import type { Highlight, Source, UserItemState, UserPreferences } from '../types.ts'

export interface Actions {
  markRead: (id: string) => void
  saveLater: (id: string) => void
  removeLater: (id: string) => void
  setProgress: (id: string, progress: number) => void
  toggleSource: (id: string) => Promise<void>
  generateAlias: (id: string) => Promise<void>
  capture: (input: { url: string; title: string; mode: 'later' | 'read' }) => Promise<boolean>
  addHighlight: (input: Omit<Highlight, 'id' | 'createdAt' | 'updatedAt'>) => Highlight
  updateHighlight: (id: string, note: string) => void
  deleteHighlight: (id: string) => void
  setPrefs: (patch: Partial<UserPreferences>) => void
  reloadSources: () => Promise<void>
}

export interface LibraryValue {
  prefs: UserPreferences
  sources: Source[]
  ready: boolean
  stateFor: (id: string, fallback: UserItemState) => UserItemState
  rememberStates: (entries: Array<[string, UserItemState]>) => void
  highlightsFor: (id: string) => Highlight[] | undefined
  hydrateHighlights: (id: string, highlights: Highlight[]) => void
  actions: Actions
}

export const LibraryContext = createContext<LibraryValue | null>(null)
