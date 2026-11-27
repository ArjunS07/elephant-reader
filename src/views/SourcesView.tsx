import { useRef, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { createSource, deleteSource, importSources, syncAll, syncSource } from '../api.ts'
import { hostname, relativeTime } from '../lib/format.ts'
import { sourceInFolder, sourceTabNames } from '../lib/folders.ts'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { Source } from '../types.ts'

const ADD = [
  { id: 'Papers', sourceType: 'papers', folder: 'Papers', openMode: 'reader' },
  { id: 'RSS', sourceType: 'rss', folder: 'RSS', openMode: 'reader' },
  { id: 'Newsletters', sourceType: 'newsletter', folder: 'Newsletters', openMode: 'reader' },
  { id: 'HN', sourceType: 'hn', folder: 'HN', openMode: 'external' },
  { id: 'Podcasts', sourceType: 'podcast', folder: 'Podcasts', openMode: 'deeplink' },
  { id: 'Video', sourceType: 'youtube', folder: 'Video', openMode: 'external' },
  { id: 'Briefings', sourceType: 'snapshot', folder: 'Briefings', openMode: 'external' },
] as const

function cue(source: Source): string {
  if (source.emailAlias) return source.emailAlias
  if (!source.feedUrl) return ''
  return hostname(source.feedUrl)
}

function youtubeHost(host: string): boolean {
  return host === 'youtube.com' || host === 'youtu.be' || host.endsWith('.youtube.com') || host.endsWith('.youtube-nocookie.com')
}

function rowDetail(source: Source): { text: string; failed: boolean } | null {
  const error = source.lastError?.trim() ?? ''
  const when = relativeTime(source.lastFetchedAt)
  let quiet = ''
  if (source.emailAlias) quiet = source.emailAlias
  else if (source.sourceType !== 'youtube' && source.feedUrl) {
    const host = hostname(source.feedUrl)
    if (host && !youtubeHost(host)) quiet = host
  }
  const parts: string[] = []
  if (error) parts.push(error)
  else if (quiet) parts.push(quiet)
  if (when) parts.push(when)
  if (!parts.length) return null
  return { text: parts.join(' · '), failed: Boolean(error) }
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
  const [tab, setTab] = useState('All')
  const [pendingDelete, setPendingDelete] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')
  const [feed, setFeed] = useState('')
  const [kind, setKind] = useState<(typeof ADD)[number]['id']>('RSS')
  const [notice, setNotice] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const needle = query.trim().toLowerCase()
  const visibleTabs = sourceTabNames(sources)
  const counts = Object.fromEntries(visibleTabs.map((name) => [name, sources.filter((source) => sourceInFolder(source, name)).length])) as Record<string, number>
  const active = visibleTabs.includes(tab) ? tab : 'All'
  const rows = sources.filter((source) => sourceInFolder(source, active) && matchesQuery(source, needle))

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
    setBusy(source.id)
    setError(null)
    try {
      await deleteSource(source.id)
      setPendingDelete((current) => (current === source.id ? null : current))
      await reloadSources()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not delete')
    } finally {
      setBusy(null)
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
          const detail = rowDetail(source)
          const confirming = pendingDelete === source.id
          return (
            <div key={source.id} className="source" data-paused={source.enabled ? 'false' : 'true'}>
              <div className="source-copy">
                <Link className="source-name" to={`/feed?source=${source.id}`}>
                  {source.name}
                </Link>
                {detail ? (
                  <span className="source-domain" data-failed={detail.failed ? 'true' : undefined}>
                    {detail.text}
                  </span>
                ) : null}
              </div>
              <div className="source-actions">
                {confirming ? (
                  <>
                    <span className="source-prompt">Delete?</span>
                    <button type="button" className="text-btn" disabled={busy === source.id} onClick={() => void remove(source)}>
                      Confirm
                    </button>
                    <button type="button" className="text-btn" disabled={busy === source.id} onClick={() => setPendingDelete(null)}>
                      Cancel
                    </button>
                  </>
                ) : (
                  <>
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
                    <button type="button" className="text-btn" onClick={() => setPendingDelete(source.id)}>
                      Delete
                    </button>
                  </>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </main>
  )
}
