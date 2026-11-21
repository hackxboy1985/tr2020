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
import { createContext, useContext, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { channelSleepPoolQueryKeys } from '../api'
import type { ChannelSleepRow } from '../types'

export type SleepPoolDialogType = 'remove-selected' | 'clear-all' | 'remove-row'

type SleepPoolContextValue = {
  open: SleepPoolDialogType | null
  setOpen: (open: SleepPoolDialogType | null) => void
  currentRow: ChannelSleepRow | null
  setCurrentRow: (row: ChannelSleepRow | null) => void
  /** Rows captured when the bulk "remove" action was triggered. */
  pendingRows: ChannelSleepRow[]
  setPendingRows: (rows: ChannelSleepRow[]) => void
  modelFilter: string
  setModelFilter: (value: string) => void
  channelFilter: string
  setChannelFilter: (value: string) => void
  resetFilters: () => void
  refresh: () => void
}

const SleepPoolContext = createContext<SleepPoolContextValue | null>(null)

export function ChannelSleepPoolProvider(props: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useState<SleepPoolDialogType | null>(null)
  const [currentRow, setCurrentRow] = useState<ChannelSleepRow | null>(null)
  const [modelFilter, setModelFilter] = useState('')
  const [channelFilter, setChannelFilter] = useState('')
  const [pendingRows, setPendingRows] = useState<ChannelSleepRow[]>([])
  const queryClient = useQueryClient()

  const value = useMemo<SleepPoolContextValue>(
    () => ({
      open,
      setOpen,
      currentRow,
      setCurrentRow,
      pendingRows,
      setPendingRows,
      modelFilter,
      setModelFilter,
      channelFilter,
      setChannelFilter,
      resetFilters: () => {
        setModelFilter('')
        setChannelFilter('')
      },
      refresh: () => {
        void queryClient.invalidateQueries({
          queryKey: channelSleepPoolQueryKeys.list(),
        })
      },
    }),
    [open, currentRow, pendingRows, modelFilter, channelFilter, queryClient]
  )

  return (
    <SleepPoolContext.Provider value={value}>
      {props.children}
    </SleepPoolContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useChannelSleepPool() {
  const ctx = useContext(SleepPoolContext)
  if (!ctx) {
    throw new Error(
      'useChannelSleepPool must be used within ChannelSleepPoolProvider'
    )
  }
  return ctx
}
