export function HeroSection() {
  return (
    <section className="pt-10 md:pt-14">
      <h1 className="text-5xl font-bold tracking-tight text-slate-200 md:text-7xl">
        Termtap<span className="text-emerald-400">.</span>
      </h1>

      <p className="mt-6 max-w-xl text-base leading-relaxed text-slate-200 md:text-lg">
        Tap into your app&apos;s API traffic from the terminal.
      </p>

      <p className="mt-4 max-w-2xl text-sm leading-6 text-slate-500">
        Observe outbound HTTP requests in real time - without modifying your code.
      </p>

      <div className="mt-8 rounded-md border border-slate-800 bg-slate-900 px-4 py-3 text-sm md:text-sm">
        <span className="text-slate-500">$ tap run -- </span>
        <span className="text-emerald-400">go run .</span>
      </div>

      <ul className="mt-7 space-y-2 text-sm text-slate-500" aria-label="Benefits">
        <li>
          <span className="mr-2 text-emerald-400">&gt;</span>
          No setup
        </li>
        <li>
          <span className="mr-2 text-emerald-400">&gt;</span>
          No instrumentation
        </li>
        <li>
          <span className="mr-2 text-emerald-400">&gt;</span>
          No guessing
        </li>
      </ul>

      <div className="mt-8 flex flex-wrap gap-3 text-sm">
        <a
          className="rounded-md border border-emerald-300 bg-emerald-300 px-5 py-2.5 text-slate-950 transition-colors hover:bg-emerald-200 focus-visible:bg-emerald-200"
          href="#install"
        >
          Install
        </a>
        <a
          className="rounded-md border border-slate-800 px-5 py-2.5 text-slate-200 transition-colors hover:border-emerald-400 focus-visible:border-emerald-400"
          href="https://github.com/haydenhargreaves/termtap"
          target="_blank"
          rel="noreferrer"
        >
          View on GitHub
        </a>
      </div>
    </section>
  )
}
