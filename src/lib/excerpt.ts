import type { Item } from '../types/model.ts'

function plain(source: string): string {
  return source
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)]\([^)]*\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/^>\s?/gm, '')
    .replace(/^[-*]\s+/gm, '')
    .replace(/[*_]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

export function excerpt(item: Item): string | null {
  const raw = item.metadata.kind === 'paper' ? item.metadata.abstract : item.plainText
  const text = plain(raw ?? '')
  if (!text) return null
  const sentences = text.split(/(?<=[.!?])\s+/).map((sentence) => sentence.trim()).filter(Boolean)
  const chosen: string[] = []
  let count = 0
  for (const sentence of sentences.slice(0, 2)) {
    const parts = sentence.split(/\s+/).filter(Boolean)
    if (chosen.length === 0 && parts.length > 35) return parts.slice(0, 32).join(' ')
    if (chosen.length > 0 && count + parts.length > 35) break
    chosen.push(sentence)
    count += parts.length
    if (count >= 25) break
  }
  return chosen.join(' ') || null
}
