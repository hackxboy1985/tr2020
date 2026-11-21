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
import { api } from '@/lib/api'
import {
  channelSleepPoolDataSchema,
  type ChannelSleepPoolData,
  type ChannelSleepPoolMutationResponse,
  type ChannelSleepPoolResponse,
} from './types'

const ENDPOINT = '/api/channel_sleep_pool/'

export const channelSleepPoolQueryKeys = {
  all: ['channel-sleep-pool'] as const,
  list: () => [...channelSleepPoolQueryKeys.all, 'list'] as const,
}

/**
 * GET /api/channel_sleep_pool/
 *
 * The payload is normalized through Zod so missing/extra fields from an older
 * or partially rolled-out backend degrade gracefully instead of crashing the
 * page.
 */
export async function getChannelSleepPool(
  signal?: AbortSignal
): Promise<ChannelSleepPoolData> {
  const res = await api.get<ChannelSleepPoolResponse>(ENDPOINT, { signal })
  const payload = res.data?.data
  if (!payload) {
    throw new Error(res.data?.message || 'Empty channel sleep pool payload')
  }
  return channelSleepPoolDataSchema.parse(payload)
}

/** DELETE /api/channel_sleep_pool/?model_name=<model>&channel_id=<id> */
export async function removeChannelSleepEntry(params: {
  modelName: string
  channelId: number
}): Promise<number> {
  const res = await api.delete<ChannelSleepPoolMutationResponse>(ENDPOINT, {
    params: {
      model_name: params.modelName,
      channel_id: params.channelId,
    },
  })
  return res.data?.data?.removed ?? 0
}

/** DELETE /api/channel_sleep_pool/?all=true */
export async function clearChannelSleepPool(): Promise<number> {
  const res = await api.delete<ChannelSleepPoolMutationResponse>(ENDPOINT, {
    params: { all: 'true' },
  })
  return res.data?.data?.removed ?? 0
}
