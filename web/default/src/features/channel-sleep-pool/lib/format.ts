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
import { getChannelTypeLabel } from '@/features/channels/lib/channel-utils'
import type { ChannelSleepEntry, ChannelSleepRow } from '../types'

/** `1m 36s` / `42s` — compact remaining time label. */
export function formatRemainingSeconds(totalSeconds: number): string {
  const seconds = Math.max(0, Math.floor(totalSeconds))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  if (rest === 0) return `${minutes}m`
  return `${minutes}m ${rest}s`
}

/** Localize a channel type id; unknown ids fall back to the raw number. */
export function formatChannelType(channelType: number): string {
  const label = getChannelTypeLabel(channelType)
  if (!label || label === 'Unknown') return `#${channelType}`
  return label
}

/**
 * The backend returns `remaining_seconds` relative to `server_time`. We
 * translate that into an absolute client timestamp once per refresh so the
 * countdown keeps ticking smoothly between polls (and stays correct even if
 * the client clock is skewed).
 */
export function toRows(entries: ChannelSleepEntry[]): ChannelSleepRow[] {
  const now = Date.now()
  return entries.map((entry) => ({
    ...entry,
    row_key: `${entry.model_name}::${entry.channel_id}`,
    expires_at: now + Math.max(0, entry.remaining_seconds) * 1000,
  }))
}

type RowWithDeadline = ChannelSleepEntry & {
  expires_at?: number
}

export function getRemainingSeconds(
  row: RowWithDeadline,
  nowMs: number
): number {
  if (typeof row.expires_at === 'number') {
    return Math.max(0, Math.ceil((row.expires_at - nowMs) / 1000))
  }
  return Math.max(0, Math.floor(row.remaining_seconds))
}

export function matchesChannelFilter(
  row: ChannelSleepEntry,
  channelFilter: string
): boolean {
  const needle = channelFilter.trim().toLowerCase()
  if (!needle) return true
  const name = row.channel_name.toLowerCase()
  const id = String(row.channel_id)
  return name.includes(needle) || id.includes(needle)
}

export function matchesModelFilter(
  row: ChannelSleepEntry,
  modelFilter: string
): boolean {
  const needle = modelFilter.trim().toLowerCase()
  if (!needle) return true
  return row.model_name.toLowerCase().includes(needle)
}
