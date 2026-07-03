import { useEffect, useRef, useState } from 'react'
import { HelpCircle } from 'lucide-react'

/** ไอคอน "?" ที่แตะ/คลิกเพื่อดูคำอธิบาย metric (ทำงานทั้งมือถือและคอม, ไม่พึ่ง dependency) */
export function MetricTooltip({ text }: { text: string }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLSpanElement>(null)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('click', onDoc)
    return () => document.removeEventListener('click', onDoc)
  }, [open])

  return (
    <span ref={ref} className="relative inline-flex">
      <button
        type="button"
        aria-label="คำอธิบาย"
        onClick={(e) => {
          e.stopPropagation()
          setOpen((o) => !o)
        }}
        className="text-muted-foreground/50 hover:text-muted-foreground transition-colors"
      >
        <HelpCircle className="size-3.5" />
      </button>
      {open && (
        <span
          role="tooltip"
          className="absolute left-1/2 top-full z-50 mt-1.5 w-52 -translate-x-1/2 rounded-md border bg-background px-2.5 py-1.5 text-[11px] font-normal normal-case leading-snug text-foreground shadow-md"
        >
          {text}
        </span>
      )}
    </span>
  )
}
