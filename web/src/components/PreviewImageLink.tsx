import { useEffect, useState } from 'react'

type PreviewImageLinkProps = {
  href: string
  src: string
  alt: string
  caption: string
}

export function PreviewImageLink({ href, src, alt, caption }: PreviewImageLinkProps) {
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (!open) return

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open])

  return (
    <figure className="mt-14 md:mt-20">
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="block w-full overflow-hidden rounded-md border border-slate-800 bg-slate-900 text-left"
      >
        <img src={src} alt={alt} className="block h-auto w-full max-w-full" />
      </button>
      <figcaption className="mt-3 text-xs text-slate-500">
        {caption} <span className="text-slate-600">(click to enlarge)</span>
      </figcaption>

      {open ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={alt}
          className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/90 p-4"
          onClick={() => setOpen(false)}
        >
          <div
            className="max-h-[92vh] max-w-[96vw] overflow-hidden rounded-md border border-slate-800 bg-slate-900 shadow-2xl"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-slate-800 px-4 py-2 text-xs text-slate-500">
              <span>{caption}</span>
              <a
                href={href}
                target="_blank"
                rel="noreferrer"
                className="text-slate-300 transition-colors hover:text-slate-100"
                onClick={(event) => event.stopPropagation()}
              >
                open image
              </a>
            </div>
            <img src={src} alt={alt} className="block max-h-[calc(92vh-2.5rem)] w-full object-contain" />
          </div>
        </div>
      ) : null}
    </figure>
  )
}
