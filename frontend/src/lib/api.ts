import { config } from "@/config";
import { getAccessToken } from "@/lib/auth";
import {
  apiErrorSchema,
  createSessionRequestSchema,
  queueStatusSchema,
  sessionListSchema,
  sessionSchema,
  type CreateSessionRequest,
  type CreateSessionResult,
  type QueueStatus,
  type Session,
} from "@/lib/schemas";

// Typed client for the backend control API (docs/reference/frontend-architecture.md §4.5).
//
// Every request carries `Authorization: Bearer <token>` — /api has NO cookie
// fallback (handoff §4). Responses are Zod-validated at the boundary.

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function authHeaders(): Promise<HeadersInit> {
  const token = await getAccessToken();
  return { Authorization: `Bearer ${token}` };
}

async function toApiError(res: Response): Promise<ApiError> {
  let code = "http_error";
  let message = res.statusText || `HTTP ${res.status}`;
  try {
    const parsed = apiErrorSchema.safeParse(await res.json());
    if (parsed.success) {
      code = parsed.data.error;
      message = parsed.data.message;
    }
  } catch {
    /* non-JSON body */
  }
  return new ApiError(res.status, code, message);
}

async function request(path: string, init: RequestInit = {}): Promise<Response> {
  const res = await fetch(`${config.apiBase}${path}`, {
    ...init,
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...(await authHeaders()),
      ...init.headers,
    },
  });
  if (!res.ok) throw await toApiError(res);
  return res;
}

export const api = {
  async listSessions(): Promise<Session[]> {
    const res = await request("/sessions");
    return sessionListSchema.parse(await res.json()).sessions;
  },

  async getSession(id: string): Promise<Session> {
    const res = await request(`/sessions/${encodeURIComponent(id)}`);
    return sessionSchema.parse(await res.json());
  },

  /**
   * POST /sessions against the hot pool: 201 hands over an already-running
   * sandbox; 202 means every warm sandbox is taken and the caller is queued
   * (follow up with getQueueStatus / streamQueueEvents).
   */
  async createSession(body: CreateSessionRequest): Promise<CreateSessionResult> {
    const payload = createSessionRequestSchema.parse(body);
    const res = await request("/sessions", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    if (res.status === 202) {
      return { outcome: "queued", queue: queueStatusSchema.parse(await res.json()) };
    }
    return { outcome: "created", session: sessionSchema.parse(await res.json()) };
  },

  /** GET /api/queue — the caller's place in line, or null when not queued. */
  async getQueueStatus(): Promise<QueueStatus | null> {
    try {
      const res = await request("/queue");
      return queueStatusSchema.parse(await res.json());
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) return null;
      throw err;
    }
  },

  async deleteSession(id: string): Promise<void> {
    await request(`/sessions/${encodeURIComponent(id)}`, { method: "DELETE" });
  },
};

// ---- SSE via fetch-stream (EventSource can't set Authorization) ----

export type SessionEvent =
  | { type: "update"; session: Session }
  | { type: "deleted"; session: Session }
  | { type: "error"; message: string };

export type QueueEvent =
  | { type: "queued"; position: number }
  | { type: "assigned"; session: Session }
  | { type: "error"; message: string };

/**
 * Shared fetch-stream SSE reader: yields (event, data) frames until the
 * signal aborts or the stream ends. Callers implement a polling fallback on
 * throw (design-principles §7).
 */
async function streamSSE(
  path: string,
  onFrame: (event: string, data: string) => void,
  signal: AbortSignal,
): Promise<void> {
  const res = await fetch(`${config.apiBase}${path}`, {
    headers: { ...(await authHeaders()), Accept: "text/event-stream" },
    signal,
  });
  if (!res.ok) throw await toApiError(res);
  if (!res.body) throw new Error("no response body for SSE stream");

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    // SSE frames are separated by a blank line.
    let sep: number;
    while ((sep = buffer.indexOf("\n\n")) !== -1) {
      const frame = buffer.slice(0, sep);
      buffer = buffer.slice(sep + 2);

      let event = "message";
      const dataLines: string[] = [];
      for (const line of frame.split("\n")) {
        if (line.startsWith(":")) continue; // heartbeat / comment
        if (line.startsWith("event:")) event = line.slice(6).trim();
        else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
      }
      if (dataLines.length > 0) onFrame(event, dataLines.join("\n"));
    }
  }
}

function parseErrorData(data: string, fallback: string): string {
  try {
    return (JSON.parse(data) as { message?: string }).message ?? fallback;
  } catch {
    return fallback;
  }
}

/** Streams GET /api/sessions/:id/events — the session's lifecycle. */
export async function streamSessionEvents(
  id: string,
  onEvent: (e: SessionEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  await streamSSE(
    `/sessions/${encodeURIComponent(id)}/events`,
    (event, data) => {
      if (event === "error") {
        onEvent({ type: "error", message: parseErrorData(data, "stream error") });
        return;
      }
      const session = sessionSchema.safeParse(JSON.parse(data));
      if (!session.success) return;
      onEvent({
        type: event === "deleted" ? "deleted" : "update",
        session: session.data,
      });
    },
    signal,
  );
}

/**
 * Streams GET /api/queue/events — live queue progress. Events: "queued"
 * (position updates) then a terminal "assigned" (with the session) or
 * "error". The backend closes the stream after the terminal event.
 */
export async function streamQueueEvents(
  onEvent: (e: QueueEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  await streamSSE(
    "/queue/events",
    (event, data) => {
      if (event === "error") {
        onEvent({ type: "error", message: parseErrorData(data, "queue stream error") });
        return;
      }
      let payload: unknown;
      try {
        payload = JSON.parse(data);
      } catch {
        return;
      }
      const obj = payload as { position?: number; session?: unknown };
      if (event === "assigned") {
        const session = sessionSchema.safeParse(obj.session);
        if (session.success) onEvent({ type: "assigned", session: session.data });
        return;
      }
      if (event === "queued" && typeof obj.position === "number") {
        onEvent({ type: "queued", position: obj.position });
      }
    },
    signal,
  );
}
