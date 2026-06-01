import React, { useState, useEffect } from 'react';
import { KubeSandboxProvider } from './context/KubeSandboxContext';
import { BackgroundCanvas } from './components/BackgroundCanvas';
import { AppShell } from './components/AppShell';
import { LandingPage } from './pages/LandingPage';
import { DashboardPage } from './pages/DashboardPage';
import { CreateSessionPage } from './pages/CreateSessionPage';
import { SessionDetailPage } from './pages/SessionDetailPage';
import { WorkspacePage } from './pages/WorkspacePage';
import { SettingsPage } from './pages/SettingsPage';

const AppContent: React.FC = () => {
  const [activePage, setActivePage] = useState<string>('landing');
  const [transitioning, setTransitioning] = useState<boolean>(false);

  // Simple client-side hash and history routing helper
  const navigate = (page: string) => {
    setTransitioning(true);
    setTimeout(() => {
      setActivePage(page);
      setTransitioning(false);
      window.scrollTo(0, 0);
    }, 150); // Easing delay matching tailwind animations
  };

  // Listen to browser state updates for seamless back navigation
  useEffect(() => {
    const handleHashChange = () => {
      const hash = window.location.hash.substring(1);
      if (hash) {
        setActivePage(hash);
      }
    };

    window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, []);

  const renderActivePage = () => {
    // Exact route matching
    if (activePage === 'landing') {
      return <LandingPage onNavigate={navigate} />;
    }
    
    if (activePage === 'dashboard') {
      return (
        <DashboardPage 
          onNavigate={navigate} 
          onSelectSession={(name) => navigate(`sessions/${name}`)} 
        />
      );
    }
    
    if (activePage === 'sessions/new') {
      return (
        <CreateSessionPage 
          onNavigate={navigate} 
          onSessionCreated={(name) => navigate(`sessions/${name}`)} 
        />
      );
    }

    if (activePage === 'settings') {
      return <SettingsPage />;
    }

    // Dynamic session details matching: sessions/:name
    if (activePage.startsWith('sessions/') && activePage.endsWith('/workspace')) {
      const parts = activePage.split('/');
      const name = parts[1];
      return <WorkspacePage sessionName={name} onNavigate={navigate} />;
    }

    if (activePage.startsWith('sessions/')) {
      const name = activePage.split('/')[1];
      return <SessionDetailPage sessionName={name} onNavigate={navigate} />;
    }

    // Fallback default
    return <LandingPage onNavigate={navigate} />;
  };

  return (
    <div className="relative min-h-screen">
      {/* 3D Moving Gradient background */}
      <BackgroundCanvas />

      {/* Primary Layout Frame */}
      <AppShell activePage={activePage} onNavigate={navigate}>
        <div className={`transition-opacity duration-200 ${transitioning ? 'opacity-0' : 'opacity-100'}`}>
          {renderActivePage()}
        </div>
      </AppShell>
    </div>
  );
};

function App() {
  return (
    <KubeSandboxProvider>
      <AppContent />
    </KubeSandboxProvider>
  );
}

export default App;
