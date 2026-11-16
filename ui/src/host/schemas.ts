import z from "zod";

import { GalaxyProduct } from "@/module/types";

const hostProductUrlSchema = z.string().optional().catch(undefined);

export const hostRuntimeConfigSchema = z.object({
  products: z
    .object({
      [GalaxyProduct.ALCHEMY]: hostProductUrlSchema,
      [GalaxyProduct.FILAMENT]: hostProductUrlSchema,
    } satisfies Record<GalaxyProduct, z.ZodType>)
    .optional()
    .catch(undefined),
});

export type HostRuntimeConfig = z.infer<typeof hostRuntimeConfigSchema>;
