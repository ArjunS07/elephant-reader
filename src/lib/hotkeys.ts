import { useEffect, useRef } from 'react'

const escapeStack: Array<() => void> = []
let escapeInstalled = false

export function installEscapeListener() {
  if (escapeInstalled || typeof window === 'undefined') return
  escapeInstalled = true
  window.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return
    const fn = escapeStack[escapeStack.length - 1]
    if (!fn) return
    event.preventDefault()
    event.stopPropagation()
    fn()
  })
}

export function useEscape(active: boolean, handler: () => void) {
  const handlerRef = useRef(handler)
  useEffect(() => {
    handlerRef.current = handler
  }, [handler])
  useEffect(() => {
    if (!active) return
    const fn = () => handlerRef.current()
    escapeStack.push(fn)
    return () => {
      const index = escapeStack.lastIndexOf(fn)
      if (index >= 0) escapeStack.splice(index, 1)
    }
  }, [active])
}

function typingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return Boolean(target.closest('input, textarea, select, button, a, [contenteditable="true"]'))
}

export function useHotkeys(map: Record<string, () => void>, enabled = true) {
  const hotkeysRef = useRef(map)
  useEffect(() => {
    hotkeysRef.current = map
  })
  useEffect(() => {
    if (!enabled) return
    const onKey = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (typingTarget(event.target)) return
      if (document.querySelector('[data-overlay]')) return
      const fn = hotkeysRef.current[event.key]
      if (!fn) return
      event.preventDefault()
      fn()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [enabled])
}
