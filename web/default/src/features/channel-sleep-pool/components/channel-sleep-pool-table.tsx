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
import { useMemo, useState } from 'react'
import {
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type SortingState,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { DataTablePage } from '@/components/data-table'
import { useNowTicker } from '../hooks/use-channel-sleep-pool-data'
import { matchesChannelFilter, matchesModelFilter } from '../lib/format'
import type { ChannelSleepRow } from '../types'
import { useChannelSleepPool } from './channel-sleep-pool-provider'
import { useChannelSleepPoolColumns } from './channel-sleep-pool-columns'
import { ChannelSleepPoolBulkActions } from './channel-sleep-pool-bulk-actions'

type ChannelSleepPoolTableProps = {
  rows: ChannelSleepRow[]
  isLoading: boolean
  isFetching: boolean
}

export function ChannelSleepPoolTable(props: ChannelSleepPoolTableProps) {
  const { t } = useTranslation()
  const { modelFilter, channelFilter } = useChannelSleepPool()
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])

  // One shared 1s ticker for the whole table — the countdown column reads it.
  const now = useNowTicker(!props.isLoading)

  const columns = useChannelSleepPoolColumns({ now })

  const filteredRows = useMemo(
    () =>
      props.rows.filter(
        (row) =>
          matchesModelFilter(row, modelFilter) &&
          matchesChannelFilter(row, channelFilter)
      ),
    [props.rows, modelFilter, channelFilter]
  )

  const table = useReactTable({
    data: filteredRows,
    columns,
    state: { sorting, rowSelection },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.row_key,
    initialState: { pagination: { pageSize: 20 } },
  })

  const isEmpty = !props.isLoading && filteredRows.length === 0
  const hasFilters = Boolean(modelFilter.trim() || channelFilter.trim())

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={props.isLoading}
      isFetching={props.isFetching}
      emptyTitle={
        hasFilters ? t('No matching sleeping channels') : t('No sleeping channels')
      }
      emptyDescription={
        hasFilters
          ? t('Try adjusting the model or channel filter.')
          : t(
              'No channel is currently sleeping. Entries appear here when a channel returns an overload status code.'
            )
      }
      skeletonKeyPrefix='channel-sleep-pool'
      showPagination={!isEmpty && filteredRows.length > 20}
      bulkActions={<ChannelSleepPoolBulkActions table={table} />}
    />
  )
}
