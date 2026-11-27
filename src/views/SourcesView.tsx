import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { IconBookmark, IconCheck } from '../components/Icons.tsx'
import { useActions, useLibrary } from '../lib/hooks.ts'
import type { SourceType } from '../types/model.ts'

const ORDER: SourceType[] = [
  'ml_digest_paper',
  'ml_digest_news',
  'rss',
  'newsletter',
  'hacker_news',
  'youtube',
  'podcast',
  'web_snapshot',
  'chrome_ext',
]

const LABEL: Record<SourceType, string> = {
  ml_digest_paper: 'Papers',
  ml_digest_news: 'News',
  rss: 'RSS',
  newsletter: 'Newsletters',
  hacker_news: 'HN',
  youtube: 'Video',
  podcast: 'Podcasts',
  web_snapshot: 'Snapshots',
  chrome_ext: 'Extension',
}

export function SourcesView() {
  const { sources } = useLibrary()
  const { toggleSource, generateAlias, capture, resetDemo } = useActions()
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [invalid, setInvalid] = useState(false)

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const submitter = (event.nativeEvent as SubmitEvent).submitter
    const mode = submitter instanceof HTMLButtonElement && submitter.value === 'read' ? 'read' : 'later'
    const ok = capture({ url, title, mode })
    if (!ok) {
      setInvalid(true)
      return
    }
    setUrl('')
    setTitle('')
    setInvalid(false)
  }

  return (
    <main className="sheet sources">
      {ORDER.map((type) => {
        const group = sources.filter((source) => source.sourceType === type)
        if (!group.length && type !== 'chrome_ext') return null
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
                      <button type="button" className="text-btn" title="Generate newsletter alias" onClick={() => generateAlias(source.id)}>
                        Alias
                      </button>
                    ) : null}
                    <button
                      type="button"
                      className="switch"
                      role="switch"
                      aria-checked={source.enabled}
                      aria-label={`${source.name} enabled`}
                      onClick={() => toggleSource(source.id)}
                    >
                      <span />
                    </button>
                  </div>
                </div>
              )
            })}
            {type === 'chrome_ext' ? (
              <form className="capture" onSubmit={submit}>
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
      <button type="button" className="reset" onClick={resetDemo}>
        Reset demo data
      </button>
    </main>
  )
}
