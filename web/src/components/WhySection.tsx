import { SectionLabel } from './SectionLabel'

export function WhySection() {
  return (
    <section className="mt-14 md:mt-20">
      <SectionLabel label="// WHY" />

      <p className="mt-6 max-w-3xl text-sm leading-7 text-slate-200 md:text-base">
        DevTools don&apos;t work for backend apps.
      </p>

      <p className="mt-4 max-w-3xl text-xs leading-6 text-slate-500 md:text-sm">
        Logs are slow and indirect. You read what the developer thought to print, not what the app
        actually did.
      </p>

      <p className="mt-4 max-w-3xl text-sm leading-7 text-slate-200 md:text-base">
        Termtap shows what your app is really doing on the network - as it happens.
      </p>
    </section>
  )
}
