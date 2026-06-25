import { mockEngine } from './mockEngine';
import type { SessionResponse, UserResponse, KubeconfigResponse, SSEEvent } from './mockEngine';

// API base URL is configured at build time via VITE_API_BASE env var
// In development it can be set in .env file, in production it's injected via Helm chart
const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1';

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
    this.name = 'ApiError';
  }
}

// Local mock mode manager
class ApiService {
  private isMockMode: boolean = false;

  constructor() {
    const forced = localStorage.getItem('kubesandbox_force_mock');
    this.isMockMode = forced === 'true'; // Default to real API; only use mock if explicitly set
  }

  public setMockMode(enabled: boolean) {
    this.isMockMode = enabled;
    localStorage.setItem('kubesandbox_force_mock', String(enabled));
  }

  public getMockMode(): boolean {
    return this.isMockMode;
  }

  private async fetchJson<T>(path: string, options?: RequestInit): Promise<T> {
    if (this.isMockMode) {
      // Simulate small random latency for realism (100-300ms)
      await new Promise(resolve => setTimeout(resolve, 150 + Math.random() * 150));
      return null as any; // Intercepted below in concrete functions
    }

    try {
      const headers = {
        'Content-Type': 'application/json',
        'X-Development-Mode': 'true', // Injects development bypass
        ...(options?.headers || {})
      };

      const res = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers
      });

      if (!res.ok) {
        let errMessage = 'API Call Failed';
        try {
          const errData = await res.json();
          errMessage = errData.error || errMessage;
        } catch (_) {}
        throw new ApiError(errMessage, res.status);
      }

      return res.json() as Promise<T>;
    } catch (err) {
      if (err instanceof ApiError) throw err;
      
      // Auto fallback to mock if connection refused
      console.warn("Backend API unreachable, automatically falling back to Mock Engine");
      this.setMockMode(true);
      throw new ApiError("Backend offline, falling back to local simulation.", 503);
    }
  }

  public async getUser(): Promise<UserResponse> {
    if (this.isMockMode) {
      return mockEngine.getUser();
    }
    try {
      return await this.fetchJson<UserResponse>('/user');
    } catch (e) {
      if (this.isMockMode) return mockEngine.getUser();
      throw e;
    }
  }

  public async getSessions(): Promise<SessionResponse[]> {
    if (this.isMockMode) {
      return mockEngine.getSessions();
    }
    try {
      const res = await this.fetchJson<{ items: SessionResponse[] }>('/sessions');
      return res.items || [];
    } catch (e) {
      if (this.isMockMode) return mockEngine.getSessions();
      throw e;
    }
  }

  public async getSession(name: string): Promise<SessionResponse> {
    if (this.isMockMode) {
      const s = mockEngine.getSession(name);
      if (!s) throw new ApiError("Session not found", 404);
      return s;
    }
    try {
      return await this.fetchJson<SessionResponse>(`/sessions/${name}`);
    } catch (e) {
      if (this.isMockMode) {
        const s = mockEngine.getSession(name);
        if (!s) throw new ApiError("Session not found", 404);
        return s;
      }
      throw e;
    }
  }

  public async createSession(req: {
    name: string;
    tenantRef: string;
    profile: 'starter' | 'standard' | 'advanced';
    ttlMinutes?: number;
    workspaceImage?: string;
  }): Promise<SessionResponse> {
    if (this.isMockMode) {
      return mockEngine.createSession(req);
    }
    try {
      return await this.fetchJson<SessionResponse>('/sessions', {
        method: 'POST',
        body: JSON.stringify(req)
      });
    } catch (e) {
      if (this.isMockMode) return mockEngine.createSession(req);
      throw e;
    }
  }

  public async deleteSession(name: string): Promise<void> {
    if (this.isMockMode) {
      return mockEngine.deleteSession(name);
    }
    try {
      await fetch(`${API_BASE}/sessions/${name}`, {
        method: 'DELETE',
        headers: { 'X-Development-Mode': 'true' }
      });
    } catch (e) {
      if (this.isMockMode) {
        mockEngine.deleteSession(name);
        return;
      }
      throw e;
    }
  }

  public async getKubeconfig(name: string): Promise<KubeconfigResponse> {
    if (this.isMockMode) {
      return mockEngine.getKubeconfig(name);
    }
    try {
      return await this.fetchJson<KubeconfigResponse>(`/sessions/${name}/kubeconfig`);
    } catch (e) {
      if (this.isMockMode) return mockEngine.getKubeconfig(name);
      throw e;
    }
  }

  // Setup Event Stream Connection
  public connectSSE(onEvent: (event: SSEEvent) => void, onError: () => void): () => void {
    if (this.isMockMode) {
      return mockEngine.subscribeToEvents(onEvent);
    }

    const eventSource = new EventSource(`${API_BASE}/sessions/events`);

    eventSource.onmessage = (e) => {
      try {
        const parsed = JSON.parse(e.data);
        // Map backend SSE message format to SSEEvent structure
        if (parsed.type === 'DELETED') {
          onEvent({ type: 'session_deleted', data: parsed.object });
        } else {
          onEvent({ type: 'session_updated', data: parsed.object });
        }
      } catch (err) {
        console.error("Error parsing SSE event data", err);
      }
    };

    eventSource.onerror = () => {
      eventSource.close();
      onError();
    };

    return () => {
      eventSource.close();
    };
  }
}

export const api = new ApiService();
