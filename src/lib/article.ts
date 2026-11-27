import type { Item, Source } from '../types.ts'
import { durationLabel, formatDate, safeUrl } from './format.ts'
import { escapeHtml, markdownToHtml } from './markdown.ts'

export function byline(item: Item, source: Source | undefined, when = item.publishedAt): string {
  const parts: string[] = []
  if (item.author) parts.push(item.author)
  if (source && source.name !== item.author) parts.push(source.name)
  const date = formatDate(when)
  if (date) parts.push(date)
  if (item.metadata.kind === 'youtube' || item.metadata.kind === 'podcast') {
    const duration = durationLabel(item.metadata.durationMin)
    if (duration) parts.push(duration)
  }
  if (item.metadata.kind === 'hn') {
    parts.push(`${item.metadata.points} pts`)
    parts.push(`${item.metadata.commentCount} comments`)
  }
  return parts.join(' · ')
}

export function articleHtml(item: Item, source: Source | undefined): string {
  if (item.metadata.kind === 'paper') {
    const paper = item.metadata
    const authors = (paper.authors ?? []).filter(Boolean).join(', ')
    const affiliation = [...(paper.affiliations ?? []), paper.venue].filter(Boolean).join(' · ')
    const links = (paper.links ?? [])
      .map((link) => {
        const label = escapeHtml(link.label)
        const href = safeUrl(link.url)
        if (!href) return `<span>${label}</span>`
        return `<a href="${escapeHtml(href)}" target="_blank" rel="noopener noreferrer">${label}</a>`
      })
      .filter(Boolean)
      .join('')
    return [
      authors ? `<p class="authors">${escapeHtml(authors)}</p>` : '',
      affiliation ? `<p class="affiliations">${escapeHtml(affiliation)}</p>` : '',
      `<p class="byline">${escapeHtml(formatDate(item.publishedAt))}</p>`,
      paper.abstract ? `<p class="abstract">${escapeHtml(paper.abstract)}</p>` : '',
      links ? `<p class="paper-links">${links}</p>` : '',
    ].join('')
  }

  const body = item.contentMarkdown
    ? markdownToHtml(item.contentMarkdown)
    : item.plainText
      ? `<p>${escapeHtml(item.plainText)}</p>`
      : ''
  return `<p class="byline">${escapeHtml(byline(item, source))}</p>${body}`
}
