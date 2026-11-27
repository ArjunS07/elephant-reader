export type SourceType =
  | 'rss'
  | 'newsletter'
  | 'youtube'
  | 'ml_digest_paper'
  | 'ml_digest_news'
  | 'podcast'
  | 'hacker_news'
  | 'web_snapshot'
  | 'chrome_ext'

export type Folder =
  | 'Papers'
  | 'Blogs'
  | 'Newsletters'
  | 'HN'
  | 'Podcasts'
  | 'Video'
  | 'Briefings'

export type OpenMode = 'reader' | 'external' | 'deeplink'

export type ThemeName = 'light' | 'warm' | 'dark'

export type FontFamily = 'inter' | 'lora' | 'newsreader'

export type ColumnWidth = 'narrow' | 'default' | 'wide'

export type ColumnAlign = 'left' | 'center'

export interface Source {
  id: string
  name: string
  sourceType: SourceType
  folder: Folder
  defaultOpenMode: OpenMode
  defaultQueue: 'firehose' | 'later'
  feedUrl?: string
  emailAlias?: string
  enabled: boolean
}

export interface PaperLink {
  label: string
  url: string
}

export interface DiscussionQuote {
  speaker: string
  text: string
}

export interface PaperMetadata {
  kind: 'paper'
  authors: string[]
  affiliations: string[]
  arxivId: string
  abstract: string
  links: PaperLink[]
  discussionQuotes: DiscussionQuote[]
}

export interface HnMetadata {
  kind: 'hn'
  points: number
  commentCount: number
}

export interface PodcastMetadata {
  kind: 'podcast'
  durationMin: number
  showName: string
}

export interface YoutubeMetadata {
  kind: 'youtube'
  channelName: string
  durationMin: number
}

export interface GenericMetadata {
  kind: 'generic'
}

export type ItemMetadata =
  | PaperMetadata
  | HnMetadata
  | PodcastMetadata
  | YoutubeMetadata
  | GenericMetadata

export interface Item {
  id: string
  sourceId: string
  externalId: string
  canonicalUrl: string
  discussionUrl?: string
  title: string
  author: string
  publishedAt: string
  openMode: OpenMode
  actionUrl: string
  contentMarkdown?: string
  plainText: string
  metadata: ItemMetadata
}

export interface UserItemState {
  inLater: boolean
  savedAt: string | null
  readAt: string | null
  readProgress: number
}

export interface Highlight {
  id: string
  itemId: string
  quote: string
  prefix: string
  suffix: string
  note: string
  createdAt: string
  updatedAt: string
}

export interface UserPreferences {
  theme: ThemeName
  fontFamily: FontFamily
  fontSize: number
  lineHeight: number
  columnWidth: ColumnWidth
  align: ColumnAlign
}

export const FOLDERS = [
  'All',
  'Papers',
  'Blogs',
  'Newsletters',
  'HN',
  'Podcasts',
  'Video',
  'Briefings',
] as const

export type FolderFilter = (typeof FOLDERS)[number]
