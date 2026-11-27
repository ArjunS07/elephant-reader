import { useRef, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { createSource, deleteSource, importSources, syncAll, syncSource } from '../api.ts'
import { hostname } from '../lib/format.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { Source } from '../types.ts'

const TABS = ['All', 'Papers', 'RSS', 'Newsletters', 'HN', 'Podcasts', 'Video', 'Briefings'] as const
type Tab = (typeof TABS)[number]

const ADD = [
  { id: 'Papers', sourceType: 'papers', folder: 'Papers', openMode: 'reader' },
  { id: 'RSS', sourceType: 'rss', folder: 'RSS', openMode: 'reader' },
  { id: 'Newsletters', sourceType: 'newsletter', folder: 'Newsletters', openMode: 'reader' },
  { id: 'HN', sourceType: 'hn', folder: 'HN', openMode: 'external' },
  { id: 'Podcasts', sourceType: 'podcast', folder: 'Podcasts', openMode: 'deeplink' },
  { id: 'Video', sourceType: 'youtube', folder: 'Video', openMode: 'external' },
  { id: 'Briefings', sourceType: 'snapshot', folder: 'Briefings', openMode: 'external' },
] as const

function inTab(source: Source, tab: Tab): boolean {
  if (tab === 'All') return true
  if (tab === 'Papers') return source.sourceType === 'papers' || source.folder === 'Papers'
  if (tab === 'RSS') return source.sourceType === 'rss'
  if (tab === 'Newsletters') return source.sourceType === 'newsletter' || source.folder === 'Newsletters'
  if (tab === 'HN') return source.sourceType === 'hn' || source.folder === 'HN'
  if (tab === 'Podcasts') return source.sourceType === 'podcast' || source.folder === 'Podcasts'
  if (tab === 'Video') return source.sourceType === 'youtube' || source.folder === 'Video'
  return source.folder === 'Briefings'
}

function cue(source: Source): string {
  if (source.emailAlias) return source.emailAlias
  if (!source.feedUrl) return ''
  return hostname(source.feedUrl)
}

function matchesQuery(source: Source, query: string): boolean {
  if (!query) return true
  const hay = [source.name, source.feedUrl, source.emailAlias, source.folder, cue(source)].join(' ').toLowerCase()
  return hay.includes(query)
}

function youtubeFeed(value: string): string {
  try {
    const parsed = new URL(value)
    const parts = parsed.pathname.split('/').filter(Boolean)
    const index = parts.indexOf('channel')
    if (index >= 0 && parts[index + 1]) {
      return `https://www.youtube.com/feeds/videos.xml?channel_id=${parts[index + 1]}`
    }
  } catch {
    return value
  }
  return value
}

export function SourcesView() {
  const { sources, ready } = useLibrary()
  const { toggleSource, reloadSources } = useActions()
  const fileRef = useRef<HTMLInputElement>(null)
  const [tab, setTab] = useState<Tab>('All')
  const [query, setQuery] = useState('')
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')
  const [feed, setFeed] = useState('')
  const [kind, setKind] = useState<(typeof ADD)[number]['id']>('RSS')
  const [notice, setNotice] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const needle = query.trim().toLowerCase()
  const counts = Object.fromEntries(TABS.map((name) => [name, sources.filter((source) => inTab(source, name)).length])) as Record<Tab, number>
  const visibleTabs = TABS.filter((name) => name === 'All' || counts[name] > 0)
  const active = visibleTabs.includes(tab) ? tab : 'All'
  const rows = sources.filter((source) => inTab(source, active) && matchesQuery(source, needle))

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const spec = ADD.find((entry) => entry.id === kind) ?? ADD[1]
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Name is required')
      return
    }
    let feedUrl = feed.trim()
    if (spec.sourceType === 'youtube' && feedUrl) feedUrl = youtubeFeed(feedUrl)
    try {
      await createSource({
        name: trimmed,
        sourceType: spec.sourceType,
        folder: spec.folder,
        defaultOpenMode: spec.openMode,
        defaultQueue: 'feed',
        feedUrl,
      })
      setName('')
      setFeed('')
      setAdding(false)
      setError(null)
      setNotice(`Added ${trimmed}`)
      await reloadSources()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not add')
    }
  }

  const onFile = async (file: File | undefined) => {
    if (!file) return
    try {
      const result = await importSources(file)
      const noun = result.added === 1 ? 'source' : 'sources'
      setNotice(`Added ${result.added} ${noun} (${result.skipped} skipped)`)
      setError(null)
      await reloadSources()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not import')
    }
  }

  const runSync = async (id?: string) => {
    setBusy(id ?? 'all')
    setError(null)
    try {
      const result = id ? await syncSource(id) : await syncAll()
      setNotice(`Synced ${result.synced}, ${result.notModified} unchanged, ${result.failed} failed, ${result.itemsAdded} new`)
      await reloadSources()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not sync')
    } finally {
      setBusy(null)
    }
  }

  const remove = async (source: Source) => {
    if (!window.confirm(`Delete ${source.name}?`)) return
    try {
      await deleteSource(source.id)
      setError(null)
      await reloadSources()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not delete')
    }
  }

  return (
    <main className="sheet sources">
      <div className="filters" role="tablist" aria-label="Source categories">
        {visibleTabs.map((name) => (
          <button key={name} type="button" role="tab" aria-current={active === name ? 'true' : undefined} onClick={() => setTab(name)}>
            {name} <span className="count">{counts[name]}</span>
          </button>
        ))}
      </div>
      <div className="source-toolbar">
        <input
          className="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Filter sources..."
          aria-label="Filter sources"
          autoCapitalize="off"
          autoCorrect="off"
          spellCheck={false}
        />
        <div className="source-toolbar-actions">
          <button type="button" className="text-btn" aria-expanded={adding} onClick={() => setAdding((value) => !value)}>
            Add source
          </button>
          <button type="button" className="text-btn" onClick={() => fileRef.current?.click()}>
            Import
          </button>
          <button type="button" className="text-btn" disabled={busy === 'all'} onClick={() => void runSync()}>
            Sync all
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,.opml,.xml,text/csv,text/xml,application/xml"
            hidden
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''
              void onFile(file)
            }}
          />
        </div>
      </div>
      {adding ? (
        <form className="add-source" onSubmit={(event) => void save(event)}>
          <input value={name} onChange={(event) => setName(event.target.value)} placeholder="Name" aria-label="Name" />
          <input
            value={feed}
            onChange={(event) => setFeed(event.target.value)}
            placeholder="Feed URL"
            aria-label="Feed URL"
            inputMode="url"
            autoCapitalize="off"
            autoCorrect="off"
            spellCheck={false}
          />
          <div className="filter-names">
            {ADD.map((entry) => (
              <button key={entry.id} type="button" aria-pressed={kind === entry.id} onClick={() => setKind(entry.id)}>
                {entry.id}
              </button>
            ))}
          </div>
          <div className="add-source-save">
            <button type="submit" className="text-btn">
              Save
            </button>
          </div>
        </form>
      ) : null}
      {!ready && !sources.length ? <p className="meta">Loading</p> : null}
      {error ? <p className="meta">{error}</p> : null}
      {notice ? <p className="meta">{notice}</p> : null}
      <div className="source-list">
        {rows.map((source) => {
          const detail = cue(source)
          return (
            <div key={source.id} className="source" data-paused={source.enabled ? 'false' : 'true'}>
              <div className="source-copy">
                <Link className="source-name" to={`/feed?source=${source.id}`}>
                  {source.name}
                </Link>
                {detail ? <span className="source-domain">{detail}</span> : null}
              </div>
              <div className="source-actions">
                <button
                  type="button"
                  className="text-btn"
                  aria-pressed={source.enabled}
                  onClick={() => void toggleSource(source.id)}
                >
                  {source.enabled ? 'Enabled' : 'Paused'}
                </button>
                <button type="button" className="text-btn" disabled={busy === source.id} onClick={() => void runSync(source.id)}>
                  Sync
                </button>
                <button type="button" className="text-btn" onClick={() => void remove(source)}>
                  Delete
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </main>
  )
}
