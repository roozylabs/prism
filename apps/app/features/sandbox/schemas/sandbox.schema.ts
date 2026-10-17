import { z } from 'zod';

export const sandboxSchema = z.object({
  model: z.string().min(1, 'Target model is required'),
  // Policy id for prism-auto; empty means the account's default policy.
  routingPolicy: z.string().default(''),
  keyPrefix: z.string().min(1, 'Gateway API Key Context is required'),
  agentId: z.string().default('default'),
  enableStream: z.boolean().default(true),
  enableAsync: z.boolean().default(false),
  userPrompt: z.string().min(1, 'Prompt / Code Instruction is required'),
});

export type SandboxFormValues = z.infer<typeof sandboxSchema>;
