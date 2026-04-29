import { useState, useEffect } from "react"
import { NavLink } from "react-router-dom"
import { cn } from "../lib/utils"
import { Play, Bot, BookOpen, Clock, BarChart3, Settings, Moon, Sun } from "lucide-react"
import { Separator } from "./ui/separator"
import { Button } from "./ui/button"

const NAV = [
  { to: "/", label: "Content", icon: Play },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/knowledge", label: "Knowledge", icon: BookOpen },
  { to: "/schedules", label: "Schedules", icon: Clock },
  { to: "/analytics", label: "Analytics", icon: BarChart3 },
  { to: "/settings", label: "Settings", icon: Settings },
]

export function Sidebar() {
  const [isDark, setIsDark] = useState(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem('theme') === 'dark'
    }
    return false
  })

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    localStorage.setItem('theme', isDark ? 'dark' : 'light')
  }, [isDark])

  return (
    <aside className="flex h-screen w-[240px] flex-col border-r bg-background sticky top-0">
      <div className="px-6 py-5">
        <span className="text-lg font-semibold tracking-tight">Ads Vance</span>
      </div>
      <Separator />
      <nav className="flex-1 space-y-1 px-3 py-4">
        {NAV.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-secondary text-foreground"
                  : "text-muted-foreground hover:bg-secondary hover:text-foreground"
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="px-3 py-4 border-t space-y-2">
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-3 text-muted-foreground"
          onClick={() => setIsDark(!isDark)}
        >
          {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          {isDark ? "Light Mode" : "Dark Mode"}
        </Button>
        <p className="text-xs text-muted-foreground px-3">v2.0 — Automated Pipeline</p>
      </div>
    </aside>
  )
}
