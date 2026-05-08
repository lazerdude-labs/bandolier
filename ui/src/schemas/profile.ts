import { z } from 'zod';
export const newClusterSchema = z.object({
  name: z.string().min(3).max(48).regex(/^[a-z][a-z0-9-]*$/, 'lowercase letters, digits, hyphens; must start with a letter'),
  profile: z.string().min(1),
});
export type NewClusterInput = z.infer<typeof newClusterSchema>;
