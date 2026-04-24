import { SectionLabel } from './SectionLabel'

export function InstallSection() {
  return (
    <section
      className="mt-14 md:mt-20"
      id="install"
    >
      <SectionLabel label="// INSTALL" />

      <p className="mt-4 text-sm text-slate-500">
        Download the binary for your OS from{' '}
        <a
          className="text-emerald-400 underline-offset-2 hover:underline"
          href="https://github.com/haydenhargreaves/termtap/releases"
        >
          GitHub Releases
        </a>
        :
      </p>

      <div className="mt-4 rounded-md border border-slate-800 bg-slate-900 px-4 py-3 text-xs md:text-sm">
        <span className="text-emerald-400">https://github.com/haydenhargreaves/termtap/releases</span>
      </div>

      <p className="mt-3 text-xs leading-6 text-slate-500">
        Choose your OS/architecture asset, unpack it, and move <code className="px-0 text-slate-200">tap </code>
        into your PATH. Run{' '}
        <code className="px-0 text-slate-200">tap cert </code>
        if you need the HTTPS trust path.
      </p>
      <p className="text-slate-600 text-xs">Supported: macOS, Linux, Windows</p>
    </section>
  )
}
