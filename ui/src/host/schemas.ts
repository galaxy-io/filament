import z from "zod";

export const hostRuntimeConfigSchema = z.object({
  alchemyUrl: z.string().optional().catch(undefined),
});

export type HostRuntimeConfig = z.infer<typeof hostRuntimeConfigSchema>;
