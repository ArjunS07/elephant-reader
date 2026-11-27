import type { ColumnAlign, ColumnWidth, FontFamily, ThemeName, UserPreferences } from '../types.ts'

export const PREFS_KEY = 'elephant.prefs'

export const COLUMN_WIDTH: Record<ColumnWidth, string> = {
  cozy: '600px',
  standard: '672px',
  wide: '760px',
}

export const defaultPrefs: UserPreferences = {
  theme: 'warm',
  fontFamily: 'inter',
  fontSize: 18,
  lineHeight: 1.65,
  columnWidth: 'standard',
  align: 'left',
}

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n))
}

function numberOr(value: unknown, fallback: number): number {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? n : fallback
}

function widthOf(value: unknown): ColumnWidth {
  if (value === 'cozy' || value === 'narrow') return 'cozy'
  if (value === 'wide') return 'wide'
  return 'standard'
}

export function sanitizePrefs(input: Partial<UserPreferences> | null | undefined): UserPreferences {
  const theme: ThemeName =
    input?.theme === 'warm' || input?.theme === 'dark' || input?.theme === 'light' ? input.theme : defaultPrefs.theme
  const fontFamily: FontFamily =
    input?.fontFamily === 'lora' || input?.fontFamily === 'newsreader' || input?.fontFamily === 'inter'
      ? input.fontFamily
      : defaultPrefs.fontFamily
  const align: ColumnAlign = input?.align === 'center' || input?.align === 'left' ? input.align : defaultPrefs.align
  const fontSize = clamp(Math.round(numberOr(input?.fontSize, defaultPrefs.fontSize)), 15, 20)
  const lineHeight = clamp(Math.round(numberOr(input?.lineHeight, defaultPrefs.lineHeight) * 20) / 20, 1.5, 1.85)
  return { theme, fontFamily, fontSize, lineHeight, columnWidth: widthOf(input?.columnWidth), align }
}

export function applyPrefs(prefs: UserPreferences) {
  const root = document.documentElement
  root.dataset.theme = prefs.theme
  root.dataset.font = prefs.fontFamily
  root.style.setProperty('--reading-size', `${prefs.fontSize}px`)
  root.style.setProperty('--reading-leading', String(prefs.lineHeight))
  root.style.setProperty('--column', COLUMN_WIDTH[prefs.columnWidth])
  root.dataset.align = prefs.align
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs))
  } catch {
    /* private mode or a full disk */
  }
}

export function loadPrefs(): UserPreferences {
  try {
    const raw = localStorage.getItem(PREFS_KEY) ?? localStorage.getItem('field.prefs')
    if (!raw) return defaultPrefs
    return sanitizePrefs(JSON.parse(raw) as Partial<UserPreferences>)
  } catch {
    return defaultPrefs
  }
}
