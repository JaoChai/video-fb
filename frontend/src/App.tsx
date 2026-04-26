import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ContentPage from './pages/Content';
import AgentsPage from './pages/Agents';
import KnowledgePage from './pages/Knowledge';
import SchedulesPage from './pages/Schedules';
import AnalyticsPage from './pages/Analytics';

const queryClient = new QueryClient();

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div style={{ display: 'flex', minHeight: '100vh', background: '#0f172a', color: '#fff' }}>
          <nav style={{ width: 220, background: '#1a3a8f', padding: '24px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
            <h2 style={{ color: '#f5851f', fontSize: 20, marginBottom: 24 }}>Ads Vance</h2>
            {[
              { to: '/', label: 'Content' },
              { to: '/agents', label: 'Agents' },
              { to: '/knowledge', label: 'Knowledge' },
              { to: '/schedules', label: 'Schedules' },
              { to: '/analytics', label: 'Analytics' },
            ].map(({ to, label }) => (
              <NavLink key={to} to={to} style={({ isActive }) => ({
                display: 'block', padding: '10px 16px', borderRadius: 8,
                background: isActive ? '#f5851f' : 'transparent',
                color: '#fff', textDecoration: 'none', fontSize: 14,
              })}>{label}</NavLink>
            ))}
          </nav>
          <main style={{ flex: 1, padding: 32, overflowY: 'auto' }}>
            <Routes>
              <Route path="/" element={<ContentPage />} />
              <Route path="/agents" element={<AgentsPage />} />
              <Route path="/knowledge" element={<KnowledgePage />} />
              <Route path="/schedules" element={<SchedulesPage />} />
              <Route path="/analytics" element={<AnalyticsPage />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
