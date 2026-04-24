import { SectionLabel } from './SectionLabel'

export function InstallSection() {
  return (
    <section
      className="mt-14 md:mt-20"
      id="install"
    >
      <SectionLabel label="// INSTALL" />

      <div className="mt-4 rounded-md border border-slate-800 bg-slate-900 px-4 py-3 text-xs md:text-sm">
        <span className="text-slate-500">$ brew install </span>
        <span className="text-emerald-400">termtap</span>
      </div>

      <p className="mt-3 text-xs leading-6 text-slate-500">
        or download a binary from{' '}
        <a className="text-slate-200 underline-offset-2 hover:underline" href="https://github.com">
          GitHub releases
        </a>
        .
      </p>
    </section>
  )
}
