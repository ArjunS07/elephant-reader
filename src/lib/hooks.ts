import { useContext } from 'react'
import { ActionsContext, LibraryContext, type Actions } from './context.ts'
import type { DataState } from './actions.ts'

export function useLibrary(): DataState {
  const value = useContext(LibraryContext)
  if (!value) throw new Error('Store missing')
  return value
}

export function useActions(): Actions {
  const value = useContext(ActionsContext)
  if (!value) throw new Error('Store missing')
  return value
}
