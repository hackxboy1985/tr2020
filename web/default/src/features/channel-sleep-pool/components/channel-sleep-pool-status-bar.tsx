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
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { ChannelSleepPoolData } from '../types'

type StatusBarProps = {
  pool: ChannelSleepPoolData | undefined
  isLoading: boolean
}

type StatusItem = {
  label: string
  value: React.ReactNode
}

function StatusTile(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='min-w-0 rounded-lg border px-3 py-2'>
      <div className='text-muted-foreground truncate text-[11px] leading-none select-none'>
        {props.label}
      </div>
      <div className='mt-1.5 min-w-0 truncate text-sm font-medium'>
        {props.children}
      </div>
    </div>
  )
}

/**
 * Read-only snapshot of the currently effective sleep-pool configuration.
 * The admin cannot change it from here — it lives under
 * Operations → System Behavior.
 */
export function ChannelSleepPoolStatusBar(props: StatusBarProps) {
  const { t } = useTranslation()

  const items: StatusItem[] = [
    {
      label: t('Overload Sleep Pool'),
      value: props.isLoading ? (
        '—'
      ) : props.pool?.enabled ? (
        <Badge variant='default'>{t('Enabled')}</Badge>
      ) : (
        <Badge variant='secondary'>{t('Disabled')}</Badge>
      ),
    },
    {
      label: t('Sleep Duration (minutes)'),
      value: props.isLoading ? '—' : (props.pool?.sleep_minutes ?? 2),
    },
    {
      label: t('Overload Status Codes'),
      value: props.isLoading ? '—' : (
        <span className='font-mono text-xs'>
          {props.pool?.status_codes || '429,503'}
        </span>
      ),
    },
    {
      label: 'Redis',
      value: props.isLoading ? (
        '—'
      ) : props.pool?.redis_enabled ? (
        <Badge variant='outline'>{t('Available')}</Badge>
      ) : (
        <Badge variant='destructive'>{t('Unavailable')}</Badge>
      ),
    },
  ]

  return (
    <div
      className={cn(
        'grid grid-cols-2 gap-2 sm:gap-3 lg:grid-cols-4',
        !props.isLoading && props.pool?.enabled === false && 'opacity-70'
      )}
    >
      {items.map((item) => (
        <StatusTile key={item.label} label={item.label}>
          {item.value}
        </StatusTile>
      ))}
    </div>
  )
}
