import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { IconBookmark, IconCheck } from '../components/Icons.tsx'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { SourceType } from '../types.ts'

const ORDER: SourceType[] = ['papers', 'rss', 'newsletter', 'hn', 'youtube', 'podcast', 'snapshot', 'manual']

const LABEL: Record<SourceType, string> = {
  papers: 'Papers',
  rss: 'RSS',
  newsletter: 'Newsletters',
  hn: 'HN',
  youtube: 'Video',
  podcast: 'Podcasts',
  snapshot: 'Snapshots',
  manual: 'Manual',
}

export function SourcesView() {
  const { sources, ready } = useLibrary()
  const { toggleSource, generateAlias, capture } = useActions()
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [invalid, setInvalid] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const submitter = (event.nativeEvent as SubmitEvent).submitter
    const mode = submitter instanceof HTMLButtonElement && submitter.value === 'read' ? 'read' : 'later'
    try {
      const ok = await capture({ url, title, mode })
      if (!ok) {
        setInvalid(true)
        return
      }
      setUrl('')
      setTitle('')
      setInvalid(false)
      setError(null)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not save')
    }
  }

  return (
    <main className="sheet sources">
      {!ready && !sources.length ? <p className="meta">Loading</p> : null}
      {error ? <p className="meta">{error}</p> : null}
      {ORDER.map((type) => {
        const group = sources.filter((source) => source.sourceType === type)
        if (!group.length && type !== 'manual') return null
        return (
          <section key={type} className="group">
            <h2>{LABEL[type]}</h2>
            {group.map((source) => {
              const detail = [source.emailAlias, source.feedUrl].filter(Boolean).join(' · ')
              return (
                <div key={source.id} className="source">
                  <div className="source-copy">
                    <Link className="source-name" to={`/feed?source=${source.id}`}>
                      {source.name}
                    </Link>
                    {detail ? <div className="meta">{detail}</div> : null}
                  </div>
                  <div className="source-actions">
                    {source.sourceType === 'newsletter' ? (
                      <button type="button" className="text-btn" title="Generate newsletter alias" onClick={() => void generateAlias(source.id)}>
                        Alias
                      </button>
                    ) : null}
                    <button
                      type="button"
                      className="switch"
                      role="switch"
                      aria-checked={source.enabled}
                      aria-label={`${source.name} enabled`}
                      onClick={() => void toggleSource(source.id)}
                    >
                      <span />
                    </button>
                  </div>
                </div>
              )
            })}
            {type === 'manual' ? (
              <form className="capture" onSubmit={(event) => void submit(event)}>
                <input
                  value={url}
                  onChange={(event) => {
                    setUrl(event.target.value)
                    setInvalid(false)
                  }}
                  placeholder="URL"
                  aria-label="URL"
                  aria-invalid={invalid}
                  inputMode="url"
                  autoCapitalize="off"
                  autoCorrect="off"
                  spellCheck={false}
                />
                <input
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  placeholder="Title"
                  aria-label="Title"
                />
                <div className="capture-actions">
                  <button type="submit" className="icon-btn" name="mode" value="later" title="Save to Later" aria-label="Save to Later">
                    <IconBookmark />
                  </button>
                  <button type="submit" className="icon-btn" name="mode" value="read" title="Mark as read" aria-label="Mark as read">
                    <IconCheck />
                  </button>
                </div>
              </form>
            ) : null}
          </section>
        )
      })}
    </main>
  )
}
