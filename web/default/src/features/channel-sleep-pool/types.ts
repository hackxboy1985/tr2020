/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { z } from 'zod'

/**
 * Contract: docs/dev/overload-sleep-contract.md §3.1
 *
 * Every numeric field is validated loosely (unknown -> coerced) so a single
 * unexpected field never breaks the whole page; the normalizer below turns
 * the raw payload into a fully typed entry.
 */
export const channelSleepNextChannelSchema = z.object({
  channel_id: z.number(),
  channel_name: z.string().default(''),
  priority: z.number().default(0),
})

export type ChannelSleepNextChannel = z.infer<
  typeof channelSleepNextChannelSchema
>

export const channelSleepEntrySchema = z.object({
  model_name: z.string(),
  channel_id: z.number(),
  channel_name: z.string().default(''),
  channel_type: z.number().default(0),
  sleep_until: z.number().default(0),
  remaining_seconds: z.number().default(0),
  trigger_status_code: z.number().default(0),
  trigger_count: z.number().default(0),
  total_channel_count: z.number().default(0),
  all_channels_sleeping: z.boolean().default(false),
  next_channels: z.array(channelSleepNextChannelSchema).default([]),
  fallback_channel: channelSleepNextChannelSchema.nullable().default(null),
})

export type ChannelSleepEntry = z.infer<typeof channelSleepEntrySchema>

export const channelSleepPoolDataSchema = z.object({
  enabled: z.boolean().default(false),
  sleep_minutes: z.number().default(2),
  status_codes: z.string().default('429,503'),
  redis_enabled: z.boolean().default(false),
  server_time: z.number().default(0),
  entries: z.array(channelSleepEntrySchema).default([]),
})

export type ChannelSleepPoolData = z.infer<typeof channelSleepPoolDataSchema>

export type ChannelSleepPoolResponse = {
  success: boolean
  message: string
  data?: ChannelSleepPoolData
}

export type ChannelSleepPoolMutationResponse = {
  success: boolean
  message: string
  data?: {
    removed: number
  }
}

/**
 * A single row in the table: the pool entry plus the identity key used for
 * selection and removal (`model_name` + `channel_id` is the natural key).
 */
export type ChannelSleepRow = ChannelSleepEntry & {
  row_key: string
  /** Client-side absolute deadline (ms) derived from `remaining_seconds`. */
  expires_at: number
}
