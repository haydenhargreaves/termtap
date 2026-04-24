import { SectionLabel } from './SectionLabel'

export function StatusSection() {
  return (
    <section
      className="mt-14 md:mt-20"
      id="docs"
    >
      <SectionLabel label="// STATUS" />

      <div className="mt-4 rounded-md border border-slate-800 bg-slate-900 p-5">
        <p className="flex items-center gap-3 text-xs md:text-sm">
          <span className="rounded-sm border border-emerald-900 px-2.5 py-1 text-emerald-300">beta</span>
          <span className="text-slate-500">v0.1 - actively developed</span>
        </p>

        <p className="mt-4 max-w-3xl text-sm leading-7 text-slate-200 md:text-base">
          Termtap is in beta. It&apos;s usable today, but still being developed and tested in the open.
        </p>

        <p className="mt-4 max-w-3xl text-xs leading-6 text-slate-500 md:text-sm">
          Expect rough edges, occasional breaking changes, and missing features. Bug reports and
          feedback on{' '}
          <a className="text-slate-200 underline-offset-2 hover:underline" href="https://github.com/haydenhargreaves/termtap">
            GitHub
          </a>{' '}
          are very welcome.
        </p>
      </div>
    </section>
  )
}
