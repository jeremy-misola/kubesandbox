import { z } from "zod";

// Runtime validation of the /api boundary. TS types are derived from the schemas
// so the client and the wire stay in sync. Mirrors backend models.Session.

export const profileSchema = z.enum(["starter", "standard", "advanced"]);
export type Profile = z.infer<typeof profileSchema>;

/** Fixed resource shapes the backend applies per profile (display only). */
export const PROFILE_RESOURCES: Record<Profile, { cpu: string; memory: string }> = {
  starter: { cpu: "250m", memory: "256Mi" },
  standard: { cpu: "500m", memory: "512Mi" },
  advanced: { cpu: "1", memory: "1Gi" },
};

export const TTL = { min: 15, max: 1440, default: 60 } as const;

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
  profile: z.string(),
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
  profile: profileSchema,
  ttlMinutes: z.number().int().min(TTL.min).max(TTL.max).optional(),
  workspaceImage: z.string().optional(),
  starterLabRef: z.string().optional(),
});
export type CreateSessionRequest = z.infer<typeof createSessionRequestSchema>;

export const apiErrorSchema = z.object({
  error: z.string(),
  message: z.string(),
});
export type ApiError = z.infer<typeof apiErrorSchema>;
