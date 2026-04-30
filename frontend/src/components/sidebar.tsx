import { useState, useEffect } from "react"
import { NavLink } from "react-router-dom"
import { cn } from "../lib/utils"
import { ROUTES } from "../lib/routes"
import {
  LayoutDashboard,
  CalendarClock,
  BarChart3,
  BookOpen,
  Bot,
  History,
  Settings,
  Moon,
  Sun,
  Menu,
  X,
} from "lucide-react"
import { Button } from "./ui/button"

export const PIPELINE_NAV = [
  { to: ROUTES.CONTENT, label: "Content", icon: LayoutDashboard },
  { to: ROUTES.SCHEDULES, label: "Schedules", icon: CalendarClock },
  { to: ROUTES.ANALYTICS, label: "Analytics", icon: BarChart3 },
]

export const CONFIG_NAV = [
  { to: ROUTES.KNOWLEDGE, label: "Knowledge", icon: BookOpen },
  { to: ROUTES.AGENTS, label: "Agents", icon: Bot },
  { to: ROUTES.PROMPT_HISTORY, label: "Prompt History", icon: History },
  { to: ROUTES.SETTINGS, label: "Settings", icon: Settings },
]

export function Sidebar() {
  const [isDark, setIsDark] = useState(() => {
    if (typeof window !== "undefined") {
      return localStorage.getItem("theme") === "dark"
    }
    return false
  })

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add("dark")
    } else {
      document.documentElement.classList.remove("dark")
    }
    localStorage.setItem("theme", isDark ? "dark" : "light")
  }, [isDark])

  return (
    <aside className="hidden md:flex h-screen w-[240px] flex-col bg-sidebar sticky top-0">
      <div className="px-6 py-5">
        <span className="text-lg font-bold tracking-tight text-sidebar-foreground">
          Ads Vance
        </span>
      </div>

      <nav className="flex-1 px-3 py-2 space-y-6">
        <NavSection label="Pipeline" items={PIPELINE_NAV} />
        <NavSection label="Configuration" items={CONFIG_NAV} />
      </nav>

      <div className="px-3 py-4 border-t border-sidebar-muted space-y-2">
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-3 text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-muted cursor-pointer"
          onClick={() => setIsDark(!isDark)}
        >
          {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          {isDark ? "Light Mode" : "Dark Mode"}
        </Button>
        <p className="text-xs text-sidebar-foreground/40 px-3">
          v2.0 — Automated Pipeline
        </p>
      </div>
    </aside>
  )
}

export function NavSection({
  label,
  items,
  onItemClick,
}: {
  label: string
  items: { to: string; label: string; icon: React.ComponentType<{ className?: string }> }[]
  onItemClick?: () => void
}) {
  return (
    <div>
      <p className="px-3 mb-2 text-[11px] font-semibold uppercase tracking-wider text-sidebar-foreground/40">
        {label}
      </p>
      <div className="space-y-0.5">
        {items.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            onClick={onItemClick}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-sidebar-accent text-accent-foreground"
                  : "text-sidebar-foreground/70 hover:bg-sidebar-muted hover:text-sidebar-foreground"
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </div>
    </div>
  )
}

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
