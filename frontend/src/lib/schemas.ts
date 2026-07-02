import { z } from "zod";

// Runtime validation of the /api boundary. TS types are derived from the schemas
// so the client and the wire stay in sync. Mirrors backend models.Session.
//
// Single sandbox type (rev 20): the starter/standard/advanced profiles are
// gone — every sandbox is identical, pre-provisioned in a hot pool, and
// handed over on create. The only knob left is the lifetime.

export const TTL = { min: 15, max: 1440, default: 60 } as const;

/** The one uniform shape every sandbox gets (display only). */
export const SANDBOX_RESOURCES = { cpu: "500m", memory: "512Mi" } as const;

export const resourcesSchema = z.object({
  cpu: z.string(),
  memory: z.string(),
});

export const sessionSchema = z.object({
  id: z.string(),
  name: z.string(),
  namespace: z.string(),
  tenantRef: z.string(),
  ownerRef: z.string(),
  ttlMinutes: z.number(),
  workspaceImage: z.string(),
  starterLabRef: z.string().optional(),
  resources: resourcesSchema,
  phase: z.string(),
  message: z.string().optional(),
  workspaceReady: z.boolean(),
  sessionNamespace: z.string().optional(),
  expiresAt: z.string().optional(),
  url: z.string().optional(),
  createdAt: z.string().optional(),
});
export type Session = z.infer<typeof sessionSchema>;

export const sessionListSchema = z.object({
  sessions: z.array(sessionSchema).nullable().transform((s) => s ?? []),
});

export const createSessionRequestSchema = z.object({
  ttlMinutes: z.number().int().min(TTL.min).max(TTL.max).optional(),
});
export type CreateSessionRequest = z.infer<typeof createSessionRequestSchema>;

// ---- warm-pool queue (backend Phase E) --------------------------------------
//
// POST /sessions returns 202 + QueueStatus when every warm sandbox is taken.
// The caller is held in a FIFO line; GET /api/queue polls the position and
// GET /api/queue/events streams it (see api.streamQueueEvents).

export const queueStatusSchema = z.object({
  status: z.literal("queued"),
  position: z.number().int().min(1),
  message: z.string().optional(),
});
export type QueueStatus = z.infer<typeof queueStatusSchema>;

/** Result of POST /sessions: an immediate hand-over or a place in line. */
export type CreateSessionResult =
  | { outcome: "created"; session: Session }
  | { outcome: "queued"; queue: QueueStatus };

export const apiErrorSchema = z.object({
  error: z.string(),
  message: z.string(),
});
export type ApiError = z.infer<typeof apiErrorSchema>;
