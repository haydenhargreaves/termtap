type SectionLabelProps = {
  label: string
}

export function SectionLabel({ label }: SectionLabelProps) {
  return <p className="text-xs tracking-widest text-slate-500">{label}</p>
}
