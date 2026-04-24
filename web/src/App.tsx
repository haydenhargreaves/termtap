import { useEffect, useState } from 'react'
import { SiteHeader } from './components/SiteHeader'
import { DocsPage } from './pages/DocsPage'
import { HomePage } from './pages/HomePage'

function App() {
  const [pathname, setPathname] = useState(() => window.location.pathname.replace(/\/$/, '') || '/')

  useEffect(() => {
    const onPopState = () => setPathname(window.location.pathname.replace(/\/$/, '') || '/')
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const navigate = (nextPath: string) => {
    const normalized = nextPath.replace(/\/$/, '') || '/'
    window.history.pushState({}, '', normalized)
    setPathname(normalized)
  }

  const isDocsPage = pathname === '/docs'

  return (
    <div className="min-h-screen bg-slate-950 text-slate-200">
      <div className="mx-auto flex min-h-screen w-full max-w-6xl flex-col border-x border-slate-800 bg-slate-950">
        <SiteHeader page={isDocsPage ? 'docs' : 'home'} onNavigate={navigate} />
        {isDocsPage ? <DocsPage /> : <HomePage />}
      </div>
    </div>
  )
}

export default App
