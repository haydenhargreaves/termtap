import { useEffect, useState, type ReactNode } from 'react'
import { PreviewImageLink } from '../components/PreviewImageLink'

const contents = [
  { id: 'install', label: 'install' },
  { id: 'quickstart', label: 'quickstart' },
  { id: 'usage', label: 'usage' },
  { id: 'examples', label: 'examples' },
  { id: 'https-certs', label: 'https & certs' },
  { id: 'privacy', label: 'privacy' },
]

type DocsSectionProps = {
  id: string
  number: string
  title: string
  children: ReactNode
}

type CommandBlockProps = {
  children: ReactNode
}

function DocsSection({ id, number, title, children }: DocsSectionProps) {
  return (
    <section id={id} className="mt-16 scroll-mt-24 first:mt-12 md:mt-20">
      <p className="text-xs text-slate-500">// {number}</p>
      <h2 className="mt-2 text-2xl font-semibold text-slate-200">{title}</h2>
      <div className="mt-6 space-y-5 text-base leading-7 text-slate-300">{children}</div>
    </section>
  )
}

function CommandBlock({ children }: CommandBlockProps) {
  return (
    <pre className="overflow-x-auto rounded-md border border-slate-800 bg-slate-900 px-4 py-3 text-sm leading-6 text-slate-300">
      {children}
    </pre>
  )
}

export function DocsPage() {
  const [activeSection, setActiveSection] = useState(contents[0]?.id ?? 'install')

  useEffect(() => {
    const sections = contents
      .map((entry) => document.getElementById(entry.id))
      .filter((section): section is HTMLElement => section !== null)

    if (!sections.length) return

    const observer = new IntersectionObserver(
      (entries) => {
        const visibleEntry = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]

        if (visibleEntry?.target.id) {
          setActiveSection(visibleEntry.target.id)
        }
      },
      {
        rootMargin: '-20% 0px -65% 0px',
        threshold: [0.1, 0.25, 0.5, 0.75, 1],
      },
    )

    sections.forEach((section) => observer.observe(section))

    const onHashChange = () => {
      const next = window.location.hash.replace('#', '')
      if (next) setActiveSection(next)
    }

    window.addEventListener('hashchange', onHashChange)

    return () => {
      observer.disconnect()
      window.removeEventListener('hashchange', onHashChange)
    }
  }, [])

  return (
    <main className="flex-1 px-5 pb-24 pt-6 md:px-8">
      <div className="mx-auto flex w-full max-w-5xl items-start gap-12">
        <aside className="sticky top-6 hidden h-fit w-52 shrink-0 self-start lg:block">
          <p className="text-xs tracking-widest text-slate-500">// CONTENTS</p>
          <nav className="mt-6 space-y-2 text-sm text-slate-500">
            {contents.map((entry) => (
              <a
                key={entry.id}
                href={`#${entry.id}`}
                className={`block border-l pl-3 transition-colors hover:text-slate-200 focus-visible:text-slate-200 ${activeSection === entry.id ? 'border-emerald-400 text-slate-200' : 'border-transparent'
                  }`}
              >
                {entry.label}
              </a>
            ))}
          </nav>
        </aside>

        <div className="min-w-0 flex-1 max-w-4xl">
          <section className="pt-6">
            <h1 className="text-4xl font-semibold tracking-tight text-slate-200 md:text-5xl">
              Docs<span className="text-emerald-400">.</span>
            </h1>
            <p className="mt-4 text-base text-slate-500">
              Everything you need to get Termtap running in under a minute.
            </p>
          </section>

          <DocsSection id="install" number="01" title="Install">
            <p>
              Download the binary for your OS from{' '}
              <a className="text-emerald-400 underline-offset-2 hover:underline" href="https://github.com/haydenhargreaves/termtap/releases">
                GitHub Releases
              </a>
              :
            </p>
            <CommandBlock>
              <span className="text-emerald-400">https://github.com/haydenhargreaves/termtap/releases</span>
            </CommandBlock>
            <p className="text-slate-500 text-sm">
              Choose your OS/architecture asset, unpack it, and move <code className="px-0 text-slate-200">tap </code>
              into your PATH. Run{' '}
              <code className="px-0 text-slate-200">tap cert </code>
              if you need the HTTPS trust path.
            </p>
            <p className="text-slate-600 text-xs">Supported: macOS, Linux, Windows</p>
            <p className="text-slate-600 text-xs">
              If demand for other install methods grows, they can be added later.
            </p>
          </DocsSection>

          <DocsSection id="quickstart" number="02" title="Quickstart">
            <p>
              Wrap any command with <span className="text-emerald-400">tap run --</span>. Termtap will boot a local
              proxy, set the right environment variables on the child process, and stream every outbound request.
            </p>
            <CommandBlock>
              <span className="text-slate-400">$ tap run -- </span>
              <span className="text-emerald-400">go run .</span>
            </CommandBlock>
            <p>You&apos;ll see a live request stream:</p>
            <PreviewImageLink
              href="/demo.png"
              src="/demo.png"
              alt="Termtap demo screenshot"
              caption="Demo screenshot preview."
            />
          </DocsSection>

          <DocsSection id="usage" number="03" title="Usage">
            <p>The general form:</p>
            <CommandBlock>
              <span className="text-slate-400">$ tap run [flags] -- &lt;your command&gt;</span>
            </CommandBlock>

            <div className="space-y-3 text-sm leading-6">
              <p className="flex flex-col gap-1 md:flex-row md:gap-8">
                <span className="w-28 text-emerald-400">--port</span>
                <span className="text-slate-300">Proxy port (default 8888)</span>
              </p>
            </div>
          </DocsSection>

          <DocsSection id="examples" number="04" title="Examples">
            <p className="text-sm text-slate-500">Go application:</p>
            <CommandBlock>
              <span className="text-slate-400">$ tap run </span>
              <span className="text-slate-400"> -- go run .</span>
            </CommandBlock>
            <p className="text-sm text-slate-500">Or use a custom port (default 8080)</p>
            <CommandBlock>
              <span className="text-slate-400">$ tap run --port </span>
              <span className="text-emerald-400">8888</span>
              <span className="text-slate-400"> -- go run .</span>
            </CommandBlock>
          </DocsSection>

          <DocsSection id="https-certs" number="05" title="HTTPS &amp; Certs">
            <p>
              Termtap inspects HTTPS traffic by terminating TLS at a local proxy. It generates a local root CA and
              prints the certificate path with <code className="px-0 text-slate-200">tap cert</code>.
            </p>
            <p className="text-slate-500">The cert lives in your user config directory and never leaves your machine.</p>

            <p className="text-sm tracking-widest text-slate-500">RUN THE CERT COMMAND</p>
            <CommandBlock>
              <span className="text-slate-400">$ tap cert</span>
            </CommandBlock>

            <p className="text-slate-500">
              The command shows the cert path, whether your system already trusts it, and OS-specific trust
              instructions if you need them.
            </p>

            <div className="rounded-md border border-slate-800 bg-slate-900 px-4 py-4">
              <p className="text-sm font-semibold text-emerald-400">NOTE</p>
              <p className="mt-2">
                Termtap uses a local CA to inspect HTTPS traffic. If an app ignores the system trust store, it may
                need to be pointed at the cert path printed by <code className="px-0 text-slate-200">tap cert</code>
                using that app&apos;s normal trust settings.
              </p>
              <p className="mt-2 text-slate-500">
                Trusting the CA only lets the app accept Termtap&apos;s certificate. Traffic still has to flow through
                the proxy for Termtap to see it.
              </p>
            </div>
          </DocsSection>

          <DocsSection id="privacy" number="06" title="Privacy">
            <p>Termtap is a local-only tool. It does not phone home, collect telemetry, or send any data anywhere.</p>
            <ul className="space-y-3">
              <li>
                <span className="mr-3 text-emerald-400">&gt;</span>
                Captured traffic stays on your machine. It is held in memory and, if you opt in with --out, written
                only to the file you specify.
              </li>
              <li>
                <span className="mr-3 text-emerald-400">&gt;</span>
                The root CA generated on install is unique to your machine and is never transmitted.
              </li>
              <li>
                <span className="mr-3 text-emerald-400">&gt;</span>
                No analytics, no crash reporting, no usage pings - not opt-out, just absent.
              </li>
              <li>
                <span className="mr-3 text-emerald-400">&gt;</span>
                Termtap makes one outbound request: an opt-in version check, only when you run `tap update`.
              </li>
              <li>
                <span className="mr-3 text-emerald-400">&gt;</span>
                The source is open. Read it, audit it, run it offline.
              </li>
            </ul>
            <p className="text-slate-500 text-sm">
              Last updated: 2026-04-19. Questions?{' '}
              <a className="text-slate-300 underline-offset-2 hover:underline" href="https://github.com/haydenhargreaves/termtap">
                Open an issue.
              </a>
            </p>
          </DocsSection>

          <div className="mt-16 flex items-center justify-between border-t border-slate-800 pt-8 text-sm text-slate-500">
            <a className="transition-colors hover:text-slate-200 focus-visible:text-slate-200" href="/">
              {'<- back home'}
            </a>
            <a
              className="transition-colors hover:text-slate-200 focus-visible:text-slate-200"
              href="https://github.com/haydenhargreaves/termtap"
              target="_blank"
              rel="noreferrer"
            >
              edit on github -&gt;
            </a>
          </div>
        </div>
      </div>
    </main>
  )
}
