import type { DiscussionQuote, Item, QuoteKind } from '../types.ts'

const WATCHLIST = new Set([
  'Lena Voss',
  'Rafael Ito',
  'Priya Raman',
  'Jonah Adler',
  'Mei-Ling Zhou',
  'Samuel Okonkwo',
  'Anika Shah',
  'Tom Berger',
  'Hanae Ito',
  'Mara Ellison',
  'Theo Park',
  'Sophie Wang',
  'Chelsea Finn',
])

export function quotesOf(item: Item): DiscussionQuote[] {
  const shared = item.discussionQuotes ?? []
  if (item.metadata.kind === 'paper') {
    return shared.length ? shared : (item.metadata.discussionQuotes ?? [])
  }
  return shared
}

export function tractionOf(item: Item): string | undefined {
  if (item.tractionNote) return item.tractionNote
  if (item.metadata.kind === 'paper') return item.metadata.tractionNote
  return undefined
}

export function labLine(item: Item): string | null {
  if (item.metadata.kind !== 'paper') return null
  const parts = [...(item.metadata.affiliations ?? [])]
  if (item.metadata.venue) parts.push(item.metadata.venue)
  return parts.length ? parts.join(' · ') : null
}

export function speakerAffiliation(quote: DiscussionQuote): string | undefined {
  if (!quote.affiliation || WATCHLIST.has(quote.speaker)) return undefined
  return quote.affiliation
}

export function kindLabel(kind: QuoteKind | undefined): string | null {
  if (kind === 'qt') return 'QT'
  if (kind === 'reply') return 'Reply'
  if (kind === 'self-reply') return 'Self-reply'
  return null
}
