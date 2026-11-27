import { createContext } from 'react'
import type { Highlight, UserPreferences } from '../types/model.ts'
import type { DataState } from './actions.ts'

export interface Actions {
  dismiss: (id: string) => void
  dismissMany: (ids: string[]) => void
  markRead: (id: string) => void
  saveLater: (id: string) => void
  setProgress: (id: string, progress: number) => void
  toggleSource: (id: string) => void
  generateAlias: (id: string) => void
  capture: (input: { url: string; title: string; mode: 'later' | 'read' }) => boolean
  addHighlight: (input: Omit<Highlight, 'id' | 'createdAt' | 'updatedAt'>) => Highlight
  updateHighlight: (id: string, note: string) => void
  deleteHighlight: (id: string) => void
  setPrefs: (patch: Partial<UserPreferences>) => void
  resetDemo: () => void
}

export const LibraryContext = createContext<DataState | null>(null)
export const ActionsContext = createContext<Actions | null>(null)
