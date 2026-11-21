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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { clearChannelSleepPool, removeChannelSleepEntry } from '../api'
import type { ChannelSleepRow } from '../types'
import { useChannelSleepPool } from './channel-sleep-pool-provider'

type PendingRemoval = {
  modelName: string
  channelId: number
}

function toPendingRemoval(row: ChannelSleepRow): PendingRemoval {
  return { modelName: row.model_name, channelId: row.channel_id }
}

export function ChannelSleepPoolDialogs() {
  const { t } = useTranslation()
  const {
    open,
    setOpen,
    currentRow,
    setCurrentRow,
    pendingRows,
    setPendingRows,
    refresh,
  } = useChannelSleepPool()
  const [isWorking, setIsWorking] = useState(false)

  const closeAll = () => {
    setOpen(null)
    setCurrentRow(null)
    setPendingRows([])
  }

  const runRemoval = async (
    targets: PendingRemoval[],
    onDone: () => void
  ) => {
    setIsWorking(true)
    try {
      let removed = 0
      for (const target of targets) {
        removed += await removeChannelSleepEntry(target)
      }
      toast.success(
        t('Removed {{count}} entry from the sleep pool', { count: removed })
      )
      refresh()
      onDone()
    } catch {
      // Errors are surfaced by the axios interceptor.
    } finally {
      setIsWorking(false)
    }
  }

  const handleRemoveSelected = async () => {
    await runRemoval(pendingRows.map(toPendingRemoval), closeAll)
  }

  const handleRemoveRow = async () => {
    if (!currentRow) return
    await runRemoval([toPendingRemoval(currentRow)], closeAll)
  }

  const handleClearAll = async () => {
    setIsWorking(true)
    try {
      const removed = await clearChannelSleepPool()
      toast.success(
        t('Cleared {{count}} entry from the sleep pool', { count: removed })
      )
      refresh()
      closeAll()
    } catch {
      // Errors are surfaced by the axios interceptor.
    } finally {
      setIsWorking(false)
    }
  }

  return (
    <>
      <ConfirmDialog
        destructive
        open={open === 'remove-selected'}
        onOpenChange={(next) => !next && closeAll()}
        isLoading={isWorking}
        className='max-w-md'
        title={t('Remove selected entries from the sleep pool?')}
        desc={t(
          '{{count}} selected (model, channel) pair(s) will stop being skipped. They can re-enter the pool on the next overload response.',
          { count: pendingRows.length }
        )}
        confirmText={t('Remove')}
        handleConfirm={handleRemoveSelected}
      />

      <ConfirmDialog
        destructive
        open={open === 'remove-row'}
        onOpenChange={(next) => !next && closeAll()}
        isLoading={isWorking}
        className='max-w-md'
        title={t('Remove from sleep pool?')}
        desc={
          currentRow
            ? t(
                'Channel #{{id}} ({{name}}) will stop being skipped for model {{model}}. It can re-enter the pool on the next overload response.',
                {
                  id: currentRow.channel_id,
                  name: currentRow.channel_name || t('Unknown'),
                  model: currentRow.model_name,
                }
              )
            : ''
        }
        confirmText={t('Remove')}
        handleConfirm={handleRemoveRow}
      />

      <ConfirmDialog
        destructive
        open={open === 'clear-all'}
        onOpenChange={(next) => !next && closeAll()}
        isLoading={isWorking}
        className='max-w-md'
        title={t('Clear the whole sleep pool?')}
        desc={t(
          'Every sleeping channel for every model will be released immediately. This cannot be undone.'
        )}
        confirmText={t('Clear All')}
        handleConfirm={handleClearAll}
      />
    </>
  )
}
