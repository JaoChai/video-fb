import { BrowserRouter, Routes, Route } from "react-router-dom"
import { ErrorBoundary } from "./components/error-boundary"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ToastProvider } from "./components/ui/toaster"
import { Sidebar } from "./components/sidebar"
import { MobileSidebar } from "./components/mobile-sidebar"
import ContentPage from "./pages/Content"
import AgentsPage from "./pages/Agents"
import KnowledgePage from "./pages/Knowledge"
import SchedulesPage from "./pages/Schedules"
import AnalyticsPage from "./pages/Analytics"
import SettingsPage from "./pages/Settings"

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <BrowserRouter>
          <div className="flex min-h-screen bg-background">
            <Sidebar />
            <div className="flex-1 flex flex-col">
              <MobileSidebar />
              <main className="flex-1 overflow-y-auto px-4 py-6 md:px-8 md:py-8 max-w-5xl">
                <ErrorBoundary>
                  <Routes>
                    <Route path="/" element={<ContentPage />} />
                    <Route path="/schedules" element={<SchedulesPage />} />
                    <Route path="/analytics" element={<AnalyticsPage />} />
                    <Route path="/knowledge" element={<KnowledgePage />} />
                    <Route path="/agents" element={<AgentsPage />} />
                    <Route path="/settings" element={<SettingsPage />} />
                  </Routes>
                </ErrorBoundary>
              </main>
            </div>
          </div>
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  )
}
