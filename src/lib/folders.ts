import type { Source } from '../types.ts'

export const CORE_FOLDERS = ['Papers', 'ML Digest', 'RSS', 'Newsletters', 'HN', 'Podcasts', 'Video', 'Briefings'] as const

const CORE = new Set<string>(CORE_FOLDERS)

export function sourceInFolder(source: Source, folder: string): boolean {
  if (folder === 'All') return true
  if (folder === 'Papers') return source.sourceType === 'papers' || source.folder === 'Papers'
  if (folder === 'ML Digest') return source.folder === 'ML Digest'
  if (folder === 'RSS') return source.sourceType === 'rss'
  if (folder === 'Newsletters') return source.sourceType === 'newsletter' || source.folder === 'Newsletters'
  if (folder === 'HN') return source.sourceType === 'hn' || source.folder === 'HN'
  if (folder === 'Podcasts') return source.sourceType === 'podcast' || source.folder === 'Podcasts'
  if (folder === 'Video') return source.sourceType === 'youtube' || source.folder === 'Video'
  if (folder === 'Briefings') return source.folder === 'Briefings'
  return source.folder === folder
}

function extraFolders(sources: Source[]): string[] {
  const folders = new Set<string>()
  for (const source of sources) {
    const folder = source.folder.trim()
    if (folder && !CORE.has(folder) && folder !== 'Blogs') folders.add(folder)
  }
  return [...folders].sort((a, b) => a.localeCompare(b))
}

export function feedFilterNames(sources: Source[], also: Source[] = []): string[] {
  const pool = [...sources, ...also]
  const populatedCore = CORE_FOLDERS.filter((name) => pool.some((source) => sourceInFolder(source, name)))
  const core = pool.length ? populatedCore : [...CORE_FOLDERS]
  return ['All', ...core, ...extraFolders(pool)]
}

export function sourceTabNames(sources: Source[]): string[] {
  return feedFilterNames(sources).filter((name) => name === 'All' || sources.some((source) => sourceInFolder(source, name)))
}
