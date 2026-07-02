import { z } from 'zod'

export const videoChannelSchema = z.object({
  id: z.number(),
  name: z.string(),
  channel_type: z.enum(['coze', 'platform']),
  base_url: z.string().default(''),
  workflow_id: z.string().default(''),
  create_path: z.string().default(''),
  status_query_path: z.string().default(''),
  groups: z.string().default('default'),
  weight: z.number().default(1),
  enabled: z.number().default(1),
  remark: z.string().default(''),
  created_at: z.number(),
  updated_at: z.number(),
})

export type VideoChannel = z.infer<typeof videoChannelSchema>

export const videoChannelFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  channel_type: z.enum(['coze', 'platform']),
  base_url: z.string().default(''),
  api_key: z.string().default(''),
  api_secret: z.string().default(''),
  workflow_id: z.string().default(''),
  create_path: z.string().default(''),
  status_query_path: z.string().default(''),
  groups: z.string().min(1, 'At least one group is required'),
  weight: z.coerce.number().min(1).default(1),
  enabled: z.number().default(1),
  remark: z.string().default(''),
})

export type VideoChannelFormValues = z.infer<typeof videoChannelFormSchema>
