type SiteHeaderProps = {
  page: 'home' | 'docs'
  onNavigate: (path: string) => void
}

const sharedLinkClasses = 'transition-colors hover:text-slate-200 focus-visible:text-slate-200'

export function SiteHeader({ page, onNavigate }: SiteHeaderProps) {
  const activeClasses = 'text-slate-200'
  const inactiveClasses = 'text-slate-500'
  const docsClasses = `${sharedLinkClasses} ${page === 'docs' ? activeClasses : inactiveClasses}`

  return (
    <header className="flex items-center justify-between px-5 pb-2 pt-6 text-xs md:px-8 md:text-sm">
      <a
        className="inline-flex items-center no-underline"
        href="/"
        onClick={(event) => {
          event.preventDefault()
          onNavigate('/')
        }}
      >
        <img src="/logo-termtap-concept-no-undertext.svg" alt="Termtap" className="h-7 w-auto md:h-8" />
      </a>
      <nav className="flex gap-4 md:gap-6">
        {page === 'home' ? (
          <a className={`${sharedLinkClasses} ${inactiveClasses}`} href="#install">
            install
          </a>
        ) : (
          <a className={`${sharedLinkClasses} ${inactiveClasses}`} href="/" onClick={(event) => {
            event.preventDefault()
            onNavigate('/')
          }}>
            home
          </a>
        )}
        <a className={docsClasses} href="/docs" onClick={(event) => {
          event.preventDefault()
          onNavigate('/docs')
        }}>
          docs
        </a>
        <a
          className={`${sharedLinkClasses} text-slate-500`}
          href="https://github.com/haydenhargreaves/termtap"
          target="_blank"
          rel="noreferrer"
        >
          github
        </a>
      </nav>
    </header>
  )
}
