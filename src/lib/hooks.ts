import { useContext } from 'react'
import { LibraryContext, type Actions, type LibraryValue } from './context.ts'

export function useLibrary(): LibraryValue {
  const value = useContext(LibraryContext)
  if (!value) throw new Error('Store missing')
  return value
}

export function useActions(): Actions {
  return useLibrary().actions
}
