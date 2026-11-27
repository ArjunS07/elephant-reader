export type SourceType = 'rss' | 'newsletter' | 'youtube' | 'hn' | 'papers' | 'podcast' | 'snapshot' | 'manual'

export type Folder = 'Papers' | 'Blogs' | 'Newsletters' | 'HN' | 'Podcasts' | 'Video' | 'Briefings'

export type OpenMode = 'reader' | 'external' | 'deeplink'

export type ThemeName = 'light' | 'warm' | 'dark'

export type FontFamily = 'inter' | 'lora' | 'newsreader'

export type ColumnWidth = 'cozy' | 'standard' | 'wide'

export type ColumnAlign = 'left' | 'center'

export interface Source {
  id: string
  name: string
  sourceType: SourceType
  folder: Folder | string
  defaultOpenMode: OpenMode
  defaultQueue: 'feed' | 'later'
  feedUrl?: string
  emailAlias?: string
  config?: Record<string, unknown>
  enabled: boolean
  lastFetchedAt?: string | null
  lastError?: string | null
  createdAt?: string
  updatedAt?: string
}

export interface PaperLink {
  label: string
  url: string
}

export type QuoteKind = 'announcement' | 'qt' | 'reply' | 'self-reply'

export interface QuoteReply {
  text: string
  url?: string
  traction?: string
}

export interface DiscussionQuote {
  speaker: string
  affiliation?: string
  text: string
  url?: string
  kind?: QuoteKind
  traction?: string
  replies?: QuoteReply[]
}

export interface PaperMetadata {
  kind: 'paper'
  authors?: string[]
  affiliations?: string[]
  arxivId?: string
  venue?: string
  tractionNote?: string
  abstract?: string
  links?: PaperLink[]
  discussionQuotes?: DiscussionQuote[]
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
  tractionNote?: string
  discussionQuotes?: DiscussionQuote[]
}

export type ItemMetadata = PaperMetadata | HnMetadata | PodcastMetadata | YoutubeMetadata | GenericMetadata

export interface Item {
  id: string
  sourceId: string
  externalId: string
  canonicalUrl: string
  discussionUrl?: string
  title: string
  author: string
  publishedAt: string
  publishedDay: string
  openMode: OpenMode
  actionUrl: string
  contentMarkdown?: string
  plainText: string
  excerpt?: string
  tractionNote?: string
  discussionQuotes?: DiscussionQuote[]
  metadata: ItemMetadata
  createdAt?: string
  updatedAt?: string
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
  updatedAt?: string
}

export interface RowModel {
  item: Item
  source: Source
  state: UserItemState
  highlightCount: number
}

export interface FeedResponse {
  day: string
  prevDay: string | null
  nextDay: string | null
  earliest: string | null
  latest: string | null
  counts: Record<string, number>
  items: RowModel[]
}
