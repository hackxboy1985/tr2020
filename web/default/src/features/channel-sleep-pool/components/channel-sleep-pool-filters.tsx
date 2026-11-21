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
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useChannelSleepPool } from './channel-sleep-pool-provider'

/**
 * Filter toolbar for the sleep pool: model name + channel name/ID, both
 * applied client-side to the already-fetched snapshot.
 */
export function ChannelSleepPoolFilters() {
  const { t } = useTranslation()
  const {
    modelFilter,
    setModelFilter,
    channelFilter,
    setChannelFilter,
    resetFilters,
  } = useChannelSleepPool()

  const isFiltered = Boolean(modelFilter.trim() || channelFilter.trim())

  return (
    <div className='flex flex-wrap items-center gap-2 sm:gap-3'>
      <Input
        placeholder={t('Filter by model name...')}
        value={modelFilter}
        onChange={(event) => setModelFilter(event.target.value)}
        className='w-full sm:w-[200px] lg:w-[240px]'
        aria-label={t('Filter by model name')}
      />
      <Input
        placeholder={t('Filter by channel name or ID...')}
        value={channelFilter}
        onChange={(event) => setChannelFilter(event.target.value)}
        className='w-full sm:w-[200px] lg:w-[240px]'
        aria-label={t('Filter by channel name or ID')}
      />
      {isFiltered && (
        <Button
          variant='ghost'
          onClick={resetFilters}
          className='text-muted-foreground hover:text-foreground gap-1 px-2'
        >
          {t('Reset')}
          <X />
        </Button>
      )}
    </div>
  )
}
