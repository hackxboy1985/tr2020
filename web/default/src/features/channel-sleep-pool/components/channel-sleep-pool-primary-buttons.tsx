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
import { Loader2, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useChannelSleepPool } from './channel-sleep-pool-provider'

type ChannelSleepPoolPrimaryButtonsProps = {
  isFetching: boolean
  canClear: boolean
}

export function ChannelSleepPoolPrimaryButtons(
  props: ChannelSleepPoolPrimaryButtonsProps
) {
  const { t } = useTranslation()
  const { setOpen, refresh } = useChannelSleepPool()

  return (
    <>
      <Button
        variant='outline'
        size='sm'
        onClick={refresh}
        disabled={props.isFetching}
      >
        {props.isFetching ? (
          <Loader2 className='animate-spin' />
        ) : (
          <RefreshCw />
        )}
        {t('Refresh')}
      </Button>
      <Button
        variant='destructive'
        size='sm'
        onClick={() => setOpen('clear-all')}
        disabled={!props.canClear}
      >
        <Trash2 />
        {t('Clear All')}
      </Button>
    </>
  )
}
