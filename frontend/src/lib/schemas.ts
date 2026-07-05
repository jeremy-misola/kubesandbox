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

// ---- guided challenges (backend challenges design) --------------------------
//
// Transcriptions of the live Go structs (content.Meta, handlers/challenges.go's
// gin.H detail response, models.ChallengeRef/GradeResult), not aspirations —
// `omitempty` on the Go side becomes `.optional()`.

export const challengeMetaSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  category: z.string(), //   rbac | networkpolicy | workloads | config |
  difficulty: z.string(), // scheduling | storage-lite | troubleshooting
  // The list path (models Meta) uses omitempty, but the detail handler builds a
  // gin.H WITHOUT omitempty — so tags can arrive as null and estMinutes as 0.
  // nullish→default keeps one schema valid against both encoders.
  estMinutes: z
    .number()
    .nullish()
    .transform((n) => n ?? 0),
  tags: z
    .array(z.string())
    .nullish()
    .transform((t) => t ?? []),
});
export type ChallengeMeta = z.infer<typeof challengeMetaSchema>;

export const challengeListSchema = z.object({
  // defensive nullable→[] like sessionListSchema
  challenges: z
    .array(challengeMetaSchema)
    .nullable()
    .transform((c) => c ?? []),
});

export const challengeStepSchema = z.object({
  id: z.string(),
  description: z.string(),
});
export type ChallengeStep = z.infer<typeof challengeStepSchema>;

export const challengeDetailSchema = challengeMetaSchema.extend({
  steps: z.array(challengeStepSchema),
  hintsTotal: z.number(),
  hints: z.array(z.string()).optional(), // present only when ?hints=n > 0
});
export type ChallengeDetail = z.infer<typeof challengeDetailSchema>;

// String on purpose (not z.enum): the gate compares === "seeded", so an unknown
// future seed state degrades to "not ready yet" (fails closed) instead of a Zod
// parse failure killing the whole session payload.
export const challengeRefSchema = z.object({
  id: z.string(),
  title: z.string().optional(), // Go omitempty
  seedState: z.string(), //         pending | seeding | seeded | failed
});
export type ChallengeRef = z.infer<typeof challengeRefSchema>;

export const gradeStepSchema = z.object({
  id: z.string(),
  description: z.string(),
  pass: z.boolean(),
  message: z.string().optional(),
});
export type GradeStep = z.infer<typeof gradeStepSchema>;

export const gradeResultSchema = z.object({
  challengeId: z.string(),
  pass: z.boolean(),
  steps: z.array(gradeStepSchema),
  gradedAt: z.string(),
});
export type GradeResult = z.infer<typeof gradeResultSchema>;

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
  // Present only when the session was created for a guided challenge. Its
  // presence is the sole discriminator for the challenge workspace; a
  // challenge-less session parses to today's object unchanged.
  challenge: challengeRefSchema.optional(),
});
export type Session = z.infer<typeof sessionSchema>;

/**
 * The one derived helper the seeding gate uses — mirrors the workspaceReady
 * idiom. A session with no challenge is trivially "seeded" (nothing to seed);
 * a challenge session is ready only when seedState === "seeded". Comparing
 * against "seeded" (not the busy states) makes an unknown future seed state
 * fail closed.
 */
export const isChallengeSeeded = (s: Session): boolean =>
  !s.challenge || s.challenge.seedState === "seeded";

export const sessionListSchema = z.object({
  sessions: z.array(sessionSchema).nullable().transform((s) => s ?? []),
});

export const createSessionRequestSchema = z.object({
  ttlMinutes: z.number().int().min(TTL.min).max(TTL.max).optional(),
  // Selects a guided challenge to seed after assignment. `.optional()` (not
  // nullable): omitted when absent, so a plain create sends the identical wire
  // body as before challenges existed.
  challengeId: z.string().optional(),
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
