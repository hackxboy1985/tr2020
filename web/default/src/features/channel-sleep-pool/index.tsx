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
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { useChannelSleepPoolData } from './hooks/use-channel-sleep-pool-data'
import { ChannelSleepPoolDialogs } from './components/channel-sleep-pool-dialogs'
import { ChannelSleepPoolFilters } from './components/channel-sleep-pool-filters'
import { ChannelSleepPoolPrimaryButtons } from './components/channel-sleep-pool-primary-buttons'
import { ChannelSleepPoolProvider } from './components/channel-sleep-pool-provider'
import { ChannelSleepPoolStatusBar } from './components/channel-sleep-pool-status-bar'
import { ChannelSleepPoolTable } from './components/channel-sleep-pool-table'

function ChannelSleepPoolContent() {
  const { t } = useTranslation()
  const { rows, pool, isLoading, isFetching, isError, error, refetch } =
    useChannelSleepPoolData()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Sleeping Models')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <ChannelSleepPoolPrimaryButtons
          isFetching={isFetching}
          canClear={rows.length > 0}
        />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex w-full flex-col gap-3 sm:gap-4'>
          <ChannelSleepPoolStatusBar pool={pool} isLoading={isLoading} />

          {isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Failed to load the sleep pool')}</AlertTitle>
              <AlertDescription className='flex flex-wrap items-center gap-2'>
                <span>
                  {error instanceof Error && error.message
                    ? error.message
                    : t('Please try again later.')}
                </span>
                <Button variant='outline' size='sm' onClick={() => refetch()}>
                  {t('Retry')}
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {!isError && (
            <>
              <ChannelSleepPoolFilters />
              <ChannelSleepPoolTable
                rows={rows}
                isLoading={isLoading}
                isFetching={isFetching}
              />
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

/**
 * Admin-only read-only view of the overload sleep pool.
 *
 * Route guard lives in `routes/_authenticated/channel-sleep-pool/index.tsx`
 * (`beforeLoad` redirects role < ROLE.ADMIN to /403), the sidebar item is
 * additionally gated by `adminOnly` on the nav entry.
 */
export function ChannelSleepPool() {
  return (
    <ChannelSleepPoolProvider>
      <ChannelSleepPoolContent />
      <ChannelSleepPoolDialogs />
    </ChannelSleepPoolProvider>
  )
}
