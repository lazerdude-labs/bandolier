import { z } from 'zod';
export const changePasswordSchema = z.object({
  current_password: z.string().min(1),
  new_password: z.string().min(12, 'must be at least 12 characters'),
  confirm_password: z.string().min(12),
}).refine((v) => v.new_password === v.confirm_password, {
  path: ['confirm_password'], message: 'must match new password',
});
export type ChangePasswordInput = z.infer<typeof changePasswordSchema>;
