import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { ApiError, createSession, getSession, onUnauthorized } from './api.ts'
import { Header } from './components/Header.tsx'
import { StoreProvider } from './lib/store.tsx'
import { FeedView } from './views/FeedView.tsx'
import { ItemView } from './views/ItemView.tsx'
import { LaterView } from './views/LaterView.tsx'
import { ReadView } from './views/ReadView.tsx'
import { SourcesView } from './views/SourcesView.tsx'

type Gate = 'checking' | 'locked' | 'open'

export default function App() {
  const [gate, setGate] = useState<Gate>('checking')

  useEffect(() => {
    document.title = 'elephant'
  }, [])

  useEffect(() => {
    let stop = false
    getSession()
      .then((status) => {
        if (stop) return
        setGate(status.authRequired && !status.authenticated ? 'locked' : 'open')
      })
      .catch((err: unknown) => {
        if (stop) return
        if (err instanceof ApiError && err.status === 401) setGate('locked')
        else setGate('open')
      })
    return () => {
      stop = true
    }
  }, [])

  useEffect(() => onUnauthorized(() => setGate('locked')), [])

  if (gate !== 'open') {
    return <Unlock checking={gate === 'checking'} onUnlock={() => setGate('open')} />
  }

  return (
    <StoreProvider>
      <Header />
      <Routes>
        <Route path="/" element={<Navigate to="/feed" replace />} />
        <Route path="/feed" element={<FeedView />} />
        <Route path="/feed/:day" element={<FeedView />} />
        <Route path="/later" element={<LaterView />} />
        <Route path="/read" element={<ReadView />} />
        <Route path="/sources" element={<SourcesView />} />
        <Route path="/item/:id" element={<ItemView />} />
        <Route path="*" element={<Navigate to="/feed" replace />} />
      </Routes>
    </StoreProvider>
  )
}

function Unlock({ checking, onUnlock }: { checking: boolean; onUnlock: () => void }) {
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (pending) return
    if (!passphrase) {
      setError('Enter the passphrase.')
      return
    }
    setPending(true)
    setError('')
    try {
      const status = await createSession(passphrase)
      if (status.authRequired && !status.authenticated) {
        setError('That passphrase did not match.')
        return
      }
      onUnlock()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) setError('That passphrase did not match.')
      else if (err instanceof ApiError && err.status === 429) setError('Too many attempts. Try again in a few minutes.')
      else setError('The library could not be reached. Try again.')
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="gate">
      <div className="gate-card">
        <div className="gate-mark">
          <svg
            viewBox="0 0 28 28"
            width="26"
            height="26"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.9"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M5 19V13.5C5 9.5 8 7 12.5 7H16.5C19.8 7 22 9.2 22 12.2V16.5C22 18.2 23 19 24.3 19C25 19 25.5 18.5 25.5 17.8" />
            <path d="M5 19H9V15.5H13.5V19H17.5V14.5" />
            <path d="M13 10.5C11.2 10.5 10.5 12 10.8 13.8C11.1 15 12.3 15.3 13.5 14.5" />
            <circle cx="18.2" cy="11.2" r="0.85" fill="currentColor" stroke="none" />
          </svg>
          elephant
        </div>
        {checking ? null : (
          <form onSubmit={submit}>
            <p>The library is locked. Enter the passphrase to continue.</p>
            <label>
              <span>Passphrase</span>
              <input
                type="password"
                name="passphrase"
                autoComplete="current-password"
                autoCapitalize="off"
                autoCorrect="off"
                spellCheck={false}
                autoFocus
                value={passphrase}
                onChange={(event) => setPassphrase(event.target.value)}
              />
            </label>
            <button type="submit" disabled={pending}>
              {pending ? 'Checking' : 'Unlock'}
            </button>
            <p className="gate-error" role="status">
              {error}
            </p>
          </form>
        )}
      </div>
    </main>
  )
}
