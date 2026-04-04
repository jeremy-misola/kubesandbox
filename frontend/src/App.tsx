import { useEffect, useMemo } from 'react';
import { AuthProvider, User } from './auth';
import HomePage from './pages/HomePage';
import TerminalPage from './pages/TerminalPage';

function getPathname() {
  return window.location.pathname.replace(/\/+$/, '') || '/';
}

/**
 * Parses user info from response headers injected by Envoy Gateway SecurityPolicy.
 * These headers are set by the claimToHeaders configuration.
 * 
 * In a production setup without a backend, these headers would need to be
 * exposed via a simple API endpoint or edge function that echoes the headers.
 * 
 * For now, we check for a meta tag or data attribute that could be set by
 * server-side rendering, or default to null for static hosting.
 */
function getUserFromHeaders(): User | null {
  // In static hosting, we can't directly access response headers from the browser.
  // Envoy Gateway injects headers on the server side, but these aren't exposed to JS.
  // 
  // For a full solution, you would either:
  // 1. Have a backend endpoint (/api/me) that returns user info from headers
  // 2. Use a Backend-for-Frontend pattern that injects user data into HTML
  // 3. Configure Envoy to add headers to a response accessible via fetch
  //
  // For development/preview purposes, check localStorage for mock user data
  const storedUser = localStorage.getItem('user');
  if (storedUser) {
    try {
      return JSON.parse(storedUser);
    } catch {
      // Invalid stored user, ignore
    }
  }
  
  return null;
}

function App() {
  useEffect(() => {
    const syncTitle = () => {
      document.title =
        getPathname() === '/terminal'
          ? 'KubeSandbox Terminal | Kubernetes at your fingertips'
          : 'KubeSandbox | Kubernetes at your fingertips';
    };

    syncTitle();
    window.addEventListener('popstate', syncTitle);
    return () => window.removeEventListener('popstate', syncTitle);
  }, []);

  // Handle OIDC callback route
  // Envoy Gateway handles the actual callback, this is just for cleanup
  useEffect(() => {
    const pathname = getPathname();
    
    // After successful OIDC callback, Envoy redirects to the original URL
    // Check if we're on the callback path and clean up URL parameters
    if (pathname === '/auth/callback') {
      // Remove query parameters from URL (state, code, etc.)
      const cleanUrl = window.location.origin + '/terminal';
      window.history.replaceState({}, '', cleanUrl);
      // The page will reload or redirect handled by Envoy
    }
  }, []);

  const pathname = getPathname();

  // Get user from headers (memoized to avoid re-computation)
  const user = useMemo(() => getUserFromHeaders(), []);

  // Render callback page (brief flash before Envoy redirects)
  if (pathname === '/auth/callback') {
    return (
      <div className="callback-page">
        <div className="callback-spinner">
          <span className="material-symbols-outlined spinning">progress_activity</span>
          <p>Completing sign in...</p>
        </div>
      </div>
    );
  }

  if (pathname === '/terminal') {
    return (
      <AuthProvider user={user}>
        <TerminalPage />
      </AuthProvider>
    );
  }

  return (
    <AuthProvider user={user}>
      <HomePage />
    </AuthProvider>
  );
}

export default App;