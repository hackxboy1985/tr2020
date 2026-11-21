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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getChannelSleepPool, channelSleepPoolQueryKeys } from '../api'
import { toRows } from '../lib/format'
import type { ChannelSleepRow } from '../types'

/** Re-fetch the pool every 15s so expired entries disappear on their own. */
const REFETCH_INTERVAL_MS = 15_000

/** Drives the per-second countdown without re-fetching. */
export function useNowTicker(enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!enabled) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [enabled])

  return now
}

export function useChannelSleepPoolData() {
  const query = useQuery({
    queryKey: channelSleepPoolQueryKeys.list(),
    queryFn: ({ signal }) => getChannelSleepPool(signal),
    refetchInterval: REFETCH_INTERVAL_MS,
    refetchOnWindowFocus: true,
  })

  // Avoid rest-spreading the query object (it re-renders on every field
  // change); expose only what the page needs.
  const rows = useMemo<ChannelSleepRow[]>(
    () => toRows(query.data?.entries ?? []),
    [query.data]
  )

  return {
    rows,
    pool: query.data,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  }
}
