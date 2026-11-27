import type { Item, Source } from '../types/model.ts'
import { formatDate } from './format.ts'
import { escapeHtml, markdownToHtml } from './markdown.ts'

export function byline(item: Item, source: Source | undefined, when = item.publishedAt): string {
  const parts: string[] = []
  if (item.author) parts.push(item.author)
  if (source && source.name !== item.author) parts.push(source.name)
  const date = formatDate(when)
  if (date) parts.push(date)
  if (item.metadata.kind === 'youtube' || item.metadata.kind === 'podcast') {
    parts.push(`${item.metadata.durationMin} min`)
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
    const links = paper.links
      .map(
        (link) =>
          `<a href="${escapeHtml(link.url)}" target="_blank" rel="noopener noreferrer">${escapeHtml(link.label)}</a>`,
      )
      .join('')
    return [
      `<p class="authors">${escapeHtml(paper.authors.join(', '))}</p>`,
      `<p class="affiliations">${escapeHtml(paper.affiliations.join(' · '))}</p>`,
      `<p class="byline">${escapeHtml(formatDate(item.publishedAt))}</p>`,
      `<p class="abstract">${escapeHtml(paper.abstract)}</p>`,
      `<p class="paper-links">${links}</p>`,
    ].join('')
  }

  const body = item.contentMarkdown
    ? markdownToHtml(item.contentMarkdown)
    : item.plainText
      ? `<p>${escapeHtml(item.plainText)}</p>`
      : ''
  return `<p class="byline">${escapeHtml(byline(item, source))}</p>${body}`
}
