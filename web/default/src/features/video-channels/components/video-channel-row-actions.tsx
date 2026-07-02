import { useState } from 'react'
import { type Row } from '@tanstack/react-table'
import { MoreHorizontal, Pencil, Power, PowerOff, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  deleteVideoChannel,
  updateVideoChannelStatus,
} from '../api'
import type { VideoChannel } from '../types'
import { useVideoChannels } from './video-channels-provider'

export function VideoChannelRowActions({ row }: { row: Row<VideoChannel> }) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow, refresh } = useVideoChannels()
  const channel = row.original
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [isToggling, setIsToggling] = useState(false)

  const handleEdit = () => {
    setCurrentRow(channel)
    setOpen('update')
  }

  const handleToggleStatus = async () => {
    setIsToggling(true)
    try {
      await updateVideoChannelStatus(
        channel.id,
        channel.enabled === 1 ? 0 : 1
      )
      refresh()
    } finally {
      setIsToggling(false)
    }
  }

  const handleDelete = async () => {
    await deleteVideoChannel(channel.id)
    refresh()
    setDeleteConfirmOpen(false)
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' size='icon' className='h-8 w-8'>
            <MoreHorizontal className='h-4 w-4' />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end'>
          <DropdownMenuItem onClick={handleEdit}>
            <Pencil className='mr-2 h-4 w-4' />
            {t('Edit')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleToggleStatus} disabled={isToggling}>
            {channel.enabled === 1 ? (
              <>
                <PowerOff className='mr-2 h-4 w-4' />
                {t('Disable')}
              </>
            ) : (
              <>
                <Power className='mr-2 h-4 w-4' />
                {t('Enable')}
              </>
            )}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => setDeleteConfirmOpen(true)}
            className='text-destructive'
          >
            <Trash2 className='mr-2 h-4 w-4' />
            {t('Delete')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={t('Delete Channel')}
        description={t('Are you sure you want to delete channel "{{name}}"?', {
          name: channel.name,
        })}
        confirmText={t('Delete')}
        destructive
        onConfirm={handleDelete}
      />
    </>
  )
}
