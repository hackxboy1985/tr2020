import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getVideoChannels, videoChannelsQueryKeys } from '../api'
import type { VideoChannel } from '../types'
import { VideoChannelRowActions } from './video-channel-row-actions'

const columnHelper = createColumnHelper<VideoChannel>()

export function VideoChannelsTable() {
  const { t } = useTranslation()
  const { data: channels = [], isLoading } = useQuery({
    queryKey: videoChannelsQueryKeys.list(),
    queryFn: getVideoChannels,
  })

  const columns = [
    columnHelper.accessor('id', {
      header: 'ID',
      cell: (info) => info.getValue(),
      size: 60,
    }),
    columnHelper.accessor('name', {
      header: t('Name'),
      cell: (info) => <span className='font-medium'>{info.getValue()}</span>,
    }),
    columnHelper.accessor('channel_type', {
      header: t('Type'),
      cell: (info) => (
        <Badge variant='outline'>
          {info.getValue() === 'coze' ? 'Coze' : 'Platform'}
        </Badge>
      ),
    }),
    columnHelper.accessor('base_url', {
      header: t('Base URL'),
      cell: (info) => (
        <span className='text-muted-foreground text-xs truncate max-w-[200px] block'>
          {info.getValue() || '—'}
        </span>
      ),
    }),
    columnHelper.accessor('groups', {
      header: t('Groups'),
      cell: (info) => (
        <div className='flex flex-wrap gap-1'>
          {info
            .getValue()
            .split(',')
            .map((g) => g.trim())
            .filter(Boolean)
            .map((g) => (
              <Badge key={g} variant='secondary' className='text-xs'>
                {g}
              </Badge>
            ))}
        </div>
      ),
    }),
    columnHelper.accessor('weight', {
      header: t('Weight'),
      cell: (info) => info.getValue(),
      size: 80,
    }),
    columnHelper.accessor('enabled', {
      header: t('Status'),
      cell: (info) =>
        info.getValue() === 1 ? (
          <Badge variant='default' className='bg-green-500'>
            {t('Enabled')}
          </Badge>
        ) : (
          <Badge variant='secondary'>{t('Disabled')}</Badge>
        ),
      size: 100,
    }),
    columnHelper.display({
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => <VideoChannelRowActions row={row} />,
      size: 80,
    }),
  ]

  const table = useReactTable({
    data: channels,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  if (isLoading) {
    return (
      <div className='text-muted-foreground py-8 text-center'>
        {t('Loading...')}
      </div>
    )
  }

  return (
    <div className='rounded-md border'>
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead
                  key={header.id}
                  style={{ width: header.getSize() }}
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.length ? (
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell
                colSpan={columns.length}
                className='h-24 text-center text-muted-foreground'
              >
                {t('No video channels configured.')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
