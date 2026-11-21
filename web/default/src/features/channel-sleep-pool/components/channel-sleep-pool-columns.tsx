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
import { useCallback, useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { ArrowRight, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { DataTableColumnHeader } from '@/components/data-table/column-header'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useChannelSleepPool } from './channel-sleep-pool-provider'
import {
  formatChannelType,
  formatRemainingSeconds,
  getRemainingSeconds,
} from '../lib/format'
import type { ChannelSleepRow } from '../types'

type UseChannelSleepPoolColumnsOptions = {
  /** Current client timestamp (ms); drives the countdown column. */
  now: number
}

export function useChannelSleepPoolColumns(
  options: UseChannelSleepPoolColumnsOptions
): ColumnDef<ChannelSleepRow>[] {
  const { t } = useTranslation()
  const { setCurrentRow, setOpen } = useChannelSleepPool()
  const onRemoveClick = useCallback(
    (row: ChannelSleepRow) => {
      setCurrentRow(row)
      setOpen('remove-row')
    },
    [setCurrentRow, setOpen]
  )

  return useMemo(() => {
    const columns: ColumnDef<ChannelSleepRow>[] = [
      {
        id: 'select',
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            indeterminate={table.getIsSomePageRowsSelected()}
            onCheckedChange={(value) =>
              table.toggleAllPageRowsSelected(!!value)
            }
            aria-label={t('Select all')}
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            aria-label={t('Select row')}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        size: 40,
        meta: { label: t('Select') },
      },
      {
        accessorKey: 'model_name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Model Name')} />
        ),
        cell: ({ row }) => (
          <span className='font-medium'>{row.original.model_name}</span>
        ),
        meta: { label: t('Model'), mobileTitle: true },
      },
      {
        accessorKey: 'channel_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Channel ID')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono text-xs'>{row.original.channel_id}</span>
        ),
        size: 100,
        meta: { label: t('Channel ID') },
      },
      {
        accessorKey: 'channel_name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Channel Name')} />
        ),
        cell: ({ row }) => (
          <span className='truncate'>{row.original.channel_name || '—'}</span>
        ),
        meta: { label: t('Channel Name') },
      },
      {
        accessorKey: 'channel_type',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Channel Type')} />
        ),
        cell: ({ row }) => (
          <Badge variant='outline'>
            {formatChannelType(row.original.channel_type)}
          </Badge>
        ),
        size: 130,
        meta: { label: t('Channel Type') },
      },
      {
        accessorKey: 'trigger_status_code',
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('Trigger Status Code')}
          />
        ),
        cell: ({ row }) => (
          <span className='font-mono text-xs'>
            {row.original.trigger_status_code}
          </span>
        ),
        size: 110,
        meta: { label: t('Trigger Status Code') },
      },
      {
        accessorKey: 'trigger_count',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Trigger Count')} />
        ),
        cell: ({ row }) => row.original.trigger_count,
        size: 100,
        meta: { label: t('Trigger Count') },
      },
      {
        id: 'remaining_seconds',
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('Remaining Seconds')}
          />
        ),
        cell: ({ row }) => {
          const remaining = getRemainingSeconds(row.original, options.now)
          if (remaining <= 0) {
            return (
              <Badge variant='secondary'>{t('Expired')}</Badge>
            )
          }
          return (
            <span className='font-mono text-xs tabular-nums'>
              {formatRemainingSeconds(remaining)}
            </span>
          )
        },
        enableSorting: false,
        size: 120,
        meta: { label: t('Remaining Seconds') },
      },
      {
        id: 'next_channel',
        header: t('Next Channel After Skip'),
        cell: ({ row }) => {
          const entry = row.original
          if (entry.all_channels_sleeping) {
            const fallback = entry.fallback_channel
            return (
              <div className='flex flex-col gap-0.5'>
                <Badge variant='destructive' className='w-fit'>
                  {t('All channels sleeping')}
                </Badge>
                <span className='text-muted-foreground text-xs'>
                  {fallback
                    ? t('Fallback to channel #{{id}} ({{name}})', {
                        id: fallback.channel_id,
                        name: fallback.channel_name || t('Unknown'),
                      })
                    : t('Takes the last channel')}
                </span>
              </div>
            )
          }

          if (entry.next_channels.length === 0) {
            return (
              <span className='text-muted-foreground text-xs'>
                {t('No candidate channel')}
              </span>
            )
          }

          return (
            <div className='flex flex-wrap items-center gap-1'>
              {entry.next_channels.map((next, index) => (
                <span key={next.channel_id} className='flex items-center gap-1'>
                  {index > 0 && (
                    <ArrowRight
                      className='text-muted-foreground size-3'
                      aria-hidden='true'
                    />
                  )}
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Badge
                          variant='outline'
                          className='cursor-default font-mono text-[11px]'
                        />
                      }
                    >
                      #{next.channel_id}
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>
                        {next.channel_name || t('Unknown')} · {t('Priority')}{' '}
                        {next.priority}
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </span>
              ))}
              {entry.total_channel_count > entry.next_channels.length && (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <span className='text-muted-foreground cursor-default text-xs' />
                    }
                  >
                    {t('{{shown}} of {{total}} channels', {
                      shown: entry.next_channels.length,
                      total: entry.total_channel_count,
                    })}
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{t('Only the first few candidate channels are listed.')}</p>
                  </TooltipContent>
                </Tooltip>
              )}
            </div>
          )
        },
        enableSorting: false,
        size: 280,
        meta: { label: t('Next Channel After Skip') },
      },
      {
        id: 'actions',
        header: t('Actions'),
        cell: ({ row }) => (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant='ghost'
                  size='icon'
                  className='text-destructive hover:text-destructive size-8'
                  onClick={() => onRemoveClick(row.original)}
                  aria-label={t('Remove from sleep pool')}
                />
              }
            >
              <Trash2 />
              <span className='sr-only'>{t('Remove from sleep pool')}</span>
            </TooltipTrigger>
            <TooltipContent>
              <p>{t('Remove from sleep pool')}</p>
            </TooltipContent>
          </Tooltip>
        ),
        enableSorting: false,
        enableHiding: false,
        size: 60,
        meta: { label: t('Actions') },
      },
    ]

    return columns
  }, [t, options, onRemoveClick])
}
