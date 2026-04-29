import { useState } from "react"
import { Menu, X } from "lucide-react"
import { PIPELINE_NAV, CONFIG_NAV, NavSection } from "./sidebar"
import { Button } from "./ui/button"

export function MobileSidebar() {
  const [open, setOpen] = useState(false)

  return (
    <>
      <header className="md:hidden sticky top-0 z-40 flex items-center gap-3 border-b bg-background px-4 py-3">
        <Button variant="ghost" size="icon" onClick={() => setOpen(true)} className="cursor-pointer">
          <Menu className="h-5 w-5" />
        </Button>
        <span className="text-sm font-semibold">Ads Vance</span>
      </header>

      {open && (
        <>
          <div className="fixed inset-0 z-50 bg-black/50" onClick={() => setOpen(false)} />
          <aside className="fixed inset-y-0 left-0 z-50 w-[280px] bg-sidebar flex flex-col">
            <div className="flex items-center justify-between px-6 py-5">
              <span className="text-lg font-bold tracking-tight text-sidebar-foreground">
                Ads Vance
              </span>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setOpen(false)}
                className="text-sidebar-foreground/60 hover:text-sidebar-foreground cursor-pointer"
              >
                <X className="h-5 w-5" />
              </Button>
            </div>
            <nav className="flex-1 px-3 py-2 space-y-6">
              <NavSection label="Pipeline" items={PIPELINE_NAV} onItemClick={() => setOpen(false)} />
              <NavSection label="Configuration" items={CONFIG_NAV} onItemClick={() => setOpen(false)} />
            </nav>
          </aside>
        </>
      )}
    </>
  )
}
