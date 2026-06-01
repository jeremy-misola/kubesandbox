export interface SessionResources {
  cpu: string;
  memory: string;
}

export interface SessionResponse {
  name: string;
  namespace: string;
  phase: 'Provisioning' | 'Ready' | 'Error' | 'Unknown';
  message: string;
  expiresAt: string;
  sessionNamespace: string;
  vclusterRelease: string;
  workspacePod: string;
  workspaceReady: boolean;
  tenantRef: string;
  ownerRef: string;
  profile: 'starter' | 'standard' | 'advanced';
  ttlMinutes: number;
  workspaceImage: string;
  resources: SessionResources;
  createdAt: string;
}

export interface UserResponse {
  id: string;
  email: string;
  name: string;
  groups: string[];
}

export interface KubeconfigResponse {
  kubeconfig: string;
  server: string;
}

export interface SSEEvent {
  type: 'session_updated' | 'session_deleted';
  data: SessionResponse;
}

type SSECallback = (event: SSEEvent) => void;

class MockEngine {
  private sessions: SessionResponse[] = [];
  private callbacks: Set<SSECallback> = new Set();
  private intervalIds: { [name: string]: any } = {};

  constructor() {
    this.loadFromStorage();
  }

  private loadFromStorage() {
    const data = localStorage.getItem('kubesandbox_mock_sessions');
    if (data) {
      try {
        this.sessions = JSON.parse(data);
        // Resume provisioning for any session stuck in provisioning
        this.sessions.forEach(s => {
          if (s.phase === 'Provisioning') {
            this.startProvisioningSequence(s.name);
          }
        });
      } catch (e) {
        this.sessions = [];
      }
    } else {
      // Seed with one default session
      this.sessions = [
        {
          name: "demo-playground",
          namespace: "playground",
          phase: "Ready",
          message: "Workspace shell is operational. Kubeconfig ready for download.",
          expiresAt: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
          sessionNamespace: "playground-demo-playground",
          vclusterRelease: "vcluster-demo-playground",
          workspacePod: "workspace-dev-pod-xyz",
          workspaceReady: true,
          tenantRef: "demo-tenant",
          ownerRef: "user@example.com",
          profile: "standard",
          ttlMinutes: 60,
          workspaceImage: "ubuntu-dev:latest",
          resources: { cpu: "2", memory: "4Gi" },
          createdAt: new Date(Date.now() - 15 * 60 * 1000).toISOString()
        }
      ];
      this.saveToStorage();
    }
  }

  private saveToStorage() {
    localStorage.setItem('kubesandbox_mock_sessions', JSON.stringify(this.sessions));
  }

  public getUser(): UserResponse {
    return {
      id: "user@example.com",
      email: "user@example.com",
      name: "Alex Sandboxer",
      groups: ["developers", "kubesandbox-users"]
    };
  }

  public getSessions(): SessionResponse[] {
    return this.sessions;
  }

  public getSession(name: string): SessionResponse | null {
    return this.sessions.find(s => s.name === name) || null;
  }

  public createSession(req: {
    name: string;
    tenantRef: string;
    profile: 'starter' | 'standard' | 'advanced';
    ttlMinutes?: number;
    workspaceImage?: string;
    resources?: SessionResources;
  }): SessionResponse {
    if (this.sessions.some(s => s.name === req.name)) {
      throw new Error(`Session ${req.name} already exists`);
    }

    const ttl = req.ttlMinutes || 60;
    const defaultResources = {
      starter: { cpu: "1", memory: "2Gi" },
      standard: { cpu: "2", memory: "4Gi" },
      advanced: { cpu: "4", memory: "8Gi" }
    };

    const newSession: SessionResponse = {
      name: req.name,
      namespace: "playground",
      phase: "Provisioning",
      message: "Allocating isolated tenant namespace in cluster...",
      expiresAt: new Date(Date.now() + ttl * 60 * 1000).toISOString(),
      sessionNamespace: `playground-${req.name}`,
      vclusterRelease: `vcluster-${req.name}`,
      workspacePod: "",
      workspaceReady: false,
      tenantRef: req.tenantRef,
      ownerRef: "user@example.com",
      profile: req.profile,
      ttlMinutes: ttl,
      workspaceImage: req.workspaceImage || "ubuntu-dev:latest",
      resources: req.resources || defaultResources[req.profile],
      createdAt: new Date().toISOString()
    };

    this.sessions.push(newSession);
    this.saveToStorage();
    this.triggerEvent('session_updated', newSession);

    this.startProvisioningSequence(req.name);

    return newSession;
  }

  private startProvisioningSequence(name: string) {
    if (this.intervalIds[name]) {
      clearTimeout(this.intervalIds[name]);
    }

    const steps = [
      { delay: 1500, message: "Spinning up vcluster control plane in namespace...", phase: "Provisioning" as const },
      { delay: 3000, message: "Deploying dev workspace pod and shell...", phase: "Provisioning" as const },
      { delay: 4500, message: "Syncing internal networking configurations...", phase: "Provisioning" as const },
      { delay: 5500, message: "Workspace shell is operational. Kubeconfig ready for download.", phase: "Ready" as const, ready: true }
    ];

    let currentStep = 0;

    const runStep = () => {
      const step = steps[currentStep];
      const sessionIndex = this.sessions.findIndex(s => s.name === name);
      if (sessionIndex === -1) return; // Deleted in the meantime

      const session = this.sessions[sessionIndex];
      session.message = step.message;
      session.phase = step.phase;
      
      if (step.ready) {
        session.workspaceReady = true;
        session.workspacePod = `workspace-dev-pod-${Math.random().toString(36).substring(2, 7)}`;
      }

      this.sessions[sessionIndex] = { ...session };
      this.saveToStorage();
      this.triggerEvent('session_updated', session);

      currentStep++;
      if (currentStep < steps.length) {
        this.intervalIds[name] = setTimeout(runStep, steps[currentStep].delay - steps[currentStep - 1].delay);
      } else {
        delete this.intervalIds[name];
      }
    };

    this.intervalIds[name] = setTimeout(runStep, steps[0].delay);
  }

  public deleteSession(name: string): void {
    if (this.intervalIds[name]) {
      clearTimeout(this.intervalIds[name]);
      delete this.intervalIds[name];
    }

    const sessionIndex = this.sessions.findIndex(s => s.name === name);
    if (sessionIndex !== -1) {
      const session = this.sessions[sessionIndex];
      this.sessions.splice(sessionIndex, 1);
      this.saveToStorage();
      this.triggerEvent('session_deleted', session);
    }
  }

  public getKubeconfig(name: string): KubeconfigResponse {
    const session = this.getSession(name);
    if (!session) throw new Error("Session not found");
    
    return {
      server: `https://vcluster-${name}.playground.kubesandbox.com`,
      kubeconfig: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==...
    server: https://vcluster-${name}.playground.kubesandbox.com
  name: kubesandbox-${name}
contexts:
- context:
    cluster: kubesandbox-${name}
    namespace: default
    user: admin
  name: kubesandbox-${name}
current-context: kubesandbox-${name}
kind: Config
preferences: {}
users:
- name: admin
  user:
    token: eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImFkbWluLXRva2VuIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlYWNjb3VudC5uYW1lIjoiYWRtaW4iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2VhY2NvdW50LnVpZCI6IjEyMzQ1NiIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmFkbWluIn0.abc123mocktoken...`
    };
  }

  public subscribeToEvents(callback: SSECallback): () => void {
    this.callbacks.add(callback);
    return () => {
      this.callbacks.delete(callback);
    };
  }

  private triggerEvent(type: 'session_updated' | 'session_deleted', data: SessionResponse) {
    const event: SSEEvent = { type, data };
    this.callbacks.forEach(cb => cb(event));
  }
}

export const mockEngine = new MockEngine();
