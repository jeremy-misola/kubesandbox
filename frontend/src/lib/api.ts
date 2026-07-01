import { config } from "@/config";
import { getAccessToken } from "@/lib/auth";
import {
  apiErrorSchema,
  createSessionRequestSchema,
  sessionListSchema,
  sessionSchema,
  type CreateSessionRequest,
  type Session,
} from "@/lib/schemas";

// Typed client for the backend control API (docs/06 §4.5).
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

  async createSession(body: CreateSessionRequest): Promise<Session> {
    const payload = createSessionRequestSchema.parse(body);
    const res = await request("/sessions", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    return sessionSchema.parse(await res.json());
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

/**
 * Streams GET /api/sessions/:id/events, parsing SSE frames manually so we can
 * attach the bearer token. Calls `onEvent` for each frame until `signal` aborts
 * or the stream ends. Callers should implement a polling fallback on throw.
 */
export async function streamSessionEvents(
  id: string,
  onEvent: (e: SessionEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  const res = await fetch(
    `${config.apiBase}/sessions/${encodeURIComponent(id)}/events`,
    {
      headers: { ...(await authHeaders()), Accept: "text/event-stream" },
      signal,
    },
  );
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
      const parsed = parseFrame(frame);
      if (parsed) onEvent(parsed);
    }
  }
}

function parseFrame(frame: string): SessionEvent | null {
  let event = "message";
  const dataLines: string[] = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith(":")) continue; // heartbeat / comment
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
  }
  if (dataLines.length === 0) return null;
  const data = dataLines.join("\n");

  if (event === "error") {
    let message = "stream error";
    try {
      message = (JSON.parse(data) as { message?: string }).message ?? message;
    } catch {
      /* keep default */
    }
    return { type: "error", message };
  }

  const session = sessionSchema.safeParse(JSON.parse(data));
  if (!session.success) return null;
  return {
    type: event === "deleted" ? "deleted" : "update",
    session: session.data,
  };
}
