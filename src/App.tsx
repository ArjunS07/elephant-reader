import { useEffect } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { Header } from './components/Header.tsx'
import { FeedView } from './views/FeedView.tsx'
import { ItemView } from './views/ItemView.tsx'
import { LaterView } from './views/LaterView.tsx'
import { ReadView } from './views/ReadView.tsx'
import { SourcesView } from './views/SourcesView.tsx'

export default function App() {
  useEffect(() => {
    document.title = 'elephant'
  }, [])

  return (
    <>
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
    </>
  )
}
