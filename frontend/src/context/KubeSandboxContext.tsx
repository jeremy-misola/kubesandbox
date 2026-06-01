import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { api, ApiError } from '../lib/api';
import type { SessionResponse, UserResponse, SSEEvent } from '../lib/mockEngine';

export type NetworkStatus = 'connected' | 'offline' | 'mocked';

interface KubeSandboxContextType {
  user: UserResponse | null;
  sessions: SessionResponse[];
  selectedSession: SessionResponse | null;
  loading: boolean;
  error: string | null;
  mockMode: boolean;
  networkStatus: NetworkStatus;
  
  fetchUser: () => Promise<void>;
  fetchSessions: () => Promise<void>;
  fetchSessionDetail: (name: string) => Promise<void>;
  createSession: (req: {
    name: string;
    tenantRef: string;
    profile: 'starter' | 'standard' | 'advanced';
    ttlMinutes: number;
    workspaceImage?: string;
  }) => Promise<SessionResponse>;
  deleteSession: (name: string) => Promise<void>;
  getKubeconfig: (name: string) => Promise<{ kubeconfig: string; server: string }>;
  toggleMockMode: (enabled: boolean) => void;
  clearError: () => void;
}

const KubeSandboxContext = createContext<KubeSandboxContextType | undefined>(undefined);

export const KubeSandboxProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<UserResponse | null>(null);
  const [sessions, setSessions] = useState<SessionResponse[]>([]);
  const [selectedSession, setSelectedSession] = useState<SessionResponse | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [mockMode, setMockMode] = useState<boolean>(api.getMockMode());
  const [networkStatus, setNetworkStatus] = useState<NetworkStatus>(
    api.getMockMode() ? 'mocked' : 'connected'
  );

  const clearError = () => setError(null);

  const toggleMockMode = useCallback((enabled: boolean) => {
    api.setMockMode(enabled);
    setMockMode(enabled);
    setNetworkStatus(enabled ? 'mocked' : 'connected');
    // Refresh data in context
    window.location.reload();
  }, []);

  const fetchUser = useCallback(async () => {
    try {
      const u = await api.getUser();
      setUser(u);
    } catch (err: any) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Could not resolve user session.");
      }
    }
  }, []);

  const fetchSessions = useCallback(async () => {
    setLoading(true);
    try {
      const s = await api.getSessions();
      setSessions(s);
      setError(null);
    } catch (err: any) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to download active sandbox sessions.");
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchSessionDetail = useCallback(async (name: string) => {
    try {
      const s = await api.getSession(name);
      setSelectedSession(s);
    } catch (err: any) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError(`Failed to retrieve session details for: ${name}`);
      }
    }
  }, []);

  const createSession = useCallback(async (req: {
    name: string;
    tenantRef: string;
    profile: 'starter' | 'standard' | 'advanced';
    ttlMinutes: number;
    workspaceImage?: string;
  }) => {
    setLoading(true);
    try {
      const s = await api.createSession(req);
      setSessions(prev => {
        if (prev.some(x => x.name === s.name)) return prev;
        return [...prev, s];
      });
      setError(null);
      return s;
    } catch (err: any) {
      const msg = err instanceof ApiError ? err.message : "Failed to trigger sandbox creation.";
      setError(msg);
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  const deleteSession = useCallback(async (name: string) => {
    try {
      await api.deleteSession(name);
      setSessions(prev => prev.filter(s => s.name !== name));
      if (selectedSession?.name === name) {
        setSelectedSession(null);
      }
    } catch (err: any) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError(`Could not submit deletion signal for sandbox: ${name}`);
      }
    }
  }, [selectedSession]);

  const getKubeconfig = useCallback(async (name: string) => {
    try {
      return await api.getKubeconfig(name);
    } catch (err: any) {
      const msg = err instanceof ApiError ? err.message : "Failed to fetch cluster credentials.";
      setError(msg);
      throw err;
    }
  }, []);

  // Initialize SSE event stream for real-time updates
  useEffect(() => {
    let cleanupSSE: (() => void) | null = null;

    const startSSE = () => {
      cleanupSSE = api.connectSSE(
        (event: SSEEvent) => {
          const { type, data } = event;
          
          if (type === 'session_updated') {
            setSessions(prev => {
              const idx = prev.findIndex(s => s.name === data.name);
              if (idx === -1) return [...prev, data];
              const next = [...prev];
              next[idx] = data;
              return next;
            });

            setSelectedSession(prev => {
              if (prev && prev.name === data.name) {
                return data;
              }
              return prev;
            });
          } else if (type === 'session_deleted') {
            setSessions(prev => prev.filter(s => s.name !== data.name));
            setSelectedSession(prev => {
              if (prev && prev.name === data.name) {
                return null;
              }
              return prev;
            });
          }
        },
        () => {
          // SSE error occurred
          console.error("SSE stream disconnected.");
          if (!mockMode) {
            setNetworkStatus('offline');
          }
        }
      );
    };

    // Load initial user identity
    fetchUser();
    // Load sessions
    fetchSessions();
    // Connect Real-time SSE
    startSSE();

    return () => {
      if (cleanupSSE) cleanupSSE();
    };
  }, [fetchUser, fetchSessions, mockMode]);

  return (
    <KubeSandboxContext.Provider
      value={{
        user,
        sessions,
        selectedSession,
        loading,
        error,
        mockMode,
        networkStatus,
        fetchUser,
        fetchSessions,
        fetchSessionDetail,
        createSession,
        deleteSession,
        getKubeconfig,
        toggleMockMode,
        clearError
      }}
    >
      {children}
    </KubeSandboxContext.Provider>
  );
};

export const useKubeSandbox = () => {
  const context = useContext(KubeSandboxContext);
  if (context === undefined) {
    throw new Error('useKubeSandbox must be used within a KubeSandboxProvider');
  }
  return context;
};
