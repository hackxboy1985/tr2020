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
import { useState, useEffect, useCallback } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/status-badge'
import { formatTimestampToDate, formatQuota, formatNumber } from '@/lib/format'
import {
  getAdminUserTopups,
  getAdminUserLogs,
  type UserTopupRecord,
  type UserLogRecord,
} from '../../api'
import type { User } from '../../types'

// ============================================================================
// Types
// ============================================================================

type TabType = 'topup' | 'consume' | 'manage'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: User | null
}

// ============================================================================
// Helpers
// ============================================================================

function TopupStatusBadge({ status, t }: { status: string; t: (k: string) => string }) {
  if (status === 'success') return <StatusBadge label={t('Success')} variant='success' copyable={false} />
  if (status === 'pending') return <StatusBadge label={t('Pending')} variant='warning' copyable={false} />
  return <StatusBadge label={t('Expired')} variant='neutral' copyable={false} />
}

function LogTypeBadge({ type, t }: { type: number; t: (k: string) => string }) {
  if (type === 1) return <StatusBadge label={t('Topup')} variant='success' copyable={false} />
  if (type === 3) return <StatusBadge label={t('Manage')} variant='neutral' copyable={false} />
  return <StatusBadge label={t('Consume')} variant='warning' copyable={false} />
}

// ============================================================================
// Pagination helper
// ============================================================================

function Pagination({
  page,
  total,
  pageSize,
  onPageChange,
  t,
}: {
  page: number
  total: number
  pageSize: number
  onPageChange: (p: number) => void
  t: (k: string) => string
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  return (
    <div className='flex items-center justify-between border-t pt-3'>
      <div className='text-muted-foreground text-xs'>
        {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} / {total}
      </div>
      <div className='flex items-center gap-2'>
        <Button variant='outline' size='sm' className='h-7 w-7 p-0' disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
          <ChevronLeft className='h-3 w-3' />
        </Button>
        <span className='text-muted-foreground text-xs'>{page} / {totalPages}</span>
        <Button variant='outline' size='sm' className='h-7 w-7 p-0' disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>
          <ChevronRight className='h-3 w-3' />
        </Button>
      </div>
    </div>
  )
}

// ============================================================================
// Main component
// ============================================================================

const PAGE_SIZE = 20

export function UserRecordsDialog({ open, onOpenChange, user }: Props) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<TabType>('topup')

  // Topup tab state
  const [topups, setTopups] = useState<UserTopupRecord[]>([])
  const [topupTotal, setTopupTotal] = useState(0)
  const [topupPage, setTopupPage] = useState(1)
  const [topupLoading, setTopupLoading] = useState(false)

  // Log tab state (consume + manage share the same list)
  const [logs, setLogs] = useState<UserLogRecord[]>([])
  const [logTotal, setLogTotal] = useState(0)
  const [logPage, setLogPage] = useState(1)
  const [logLoading, setLogLoading] = useState(false)

  const logType = tab === 'consume' ? 2 : tab === 'manage' ? 3 : 1

  const fetchTopups = useCallback(async (uid: number, page: number) => {
    setTopupLoading(true)
    try {
      const res = await getAdminUserTopups(uid, page, PAGE_SIZE)
      if (res.success && res.data) {
        setTopups(res.data.items || [])
        setTopupTotal(res.data.total || 0)
      } else {
        toast.error(res.message || t('Failed to load records'))
      }
    } catch {
      toast.error(t('Failed to load records'))
    } finally {
      setTopupLoading(false)
    }
  }, [t])

  const fetchLogs = useCallback(async (username: string, page: number, type: number) => {
    setLogLoading(true)
    try {
      const res = await getAdminUserLogs(username, type, page, PAGE_SIZE)
      if (res.success && res.data) {
        setLogs(res.data.items || [])
        setLogTotal(res.data.total || 0)
      } else {
        toast.error(res.message || t('Failed to load records'))
      }
    } catch {
      toast.error(t('Failed to load records'))
    } finally {
      setLogLoading(false)
    }
  }, [t])

  useEffect(() => {
    if (!open || !user) return
    if (tab === 'topup') {
      fetchTopups(user.id, topupPage)
    } else {
      fetchLogs(user.username, logPage, logType)
    }
  }, [open, user, tab, topupPage, logPage, logType, fetchTopups, fetchLogs])

  // Reset pages when switching tabs
  const handleTabChange = (newTab: TabType) => {
    setTab(newTab)
    setTopupPage(1)
    setLogPage(1)
  }

  if (!user) return null

  const quotaBalance = user.quota ?? 0
  // Total topup is not directly available on user object — we show balance + used
  const usedQuota = user.used_quota ?? 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('User Records')}</DialogTitle>
          <DialogDescription>
            {user.username} (ID: {user.id})
          </DialogDescription>
        </DialogHeader>

        {/* Stats bar */}
        <div className='grid grid-cols-2 gap-3 rounded-lg border p-3 text-sm sm:grid-cols-2'>
          <div>
            <div className='text-muted-foreground text-xs'>{t('Balance')}</div>
            <div className='mt-0.5 font-semibold'>{formatQuota(quotaBalance)}</div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>{t('Total Used')}</div>
            <div className='mt-0.5 font-semibold'>{formatQuota(usedQuota)}</div>
          </div>
        </div>

        {/* Tabs */}
        <div className='flex gap-1 rounded-md border p-1'>
          {(['topup', 'consume', 'manage'] as TabType[]).map((tabKey) => {
            const labels: Record<TabType, string> = {
              topup: t('Topup Records'),
              consume: t('Consume Records'),
              manage: t('Quota Adjustments'),
            }
            return (
              <button
                key={tabKey}
                onClick={() => handleTabChange(tabKey)}
                className={`flex-1 rounded px-3 py-1.5 text-sm transition-colors ${
                  tab === tabKey
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted'
                }`}
              >
                {labels[tabKey]}
              </button>
            )
          })}
        </div>

        {/* Content */}
        <ScrollArea className='flex-1 overflow-auto' style={{ maxHeight: '420px' }}>
          {tab === 'topup' ? (
            topupLoading ? (
              <div className='space-y-2'>
                {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className='h-14 w-full rounded-lg' />)}
              </div>
            ) : topups.length === 0 ? (
              <div className='text-muted-foreground py-12 text-center text-sm'>{t('No records')}</div>
            ) : (
              <div className='space-y-2 pr-2'>
                {topups.map((r) => (
                  <div key={r.id} className='rounded-lg border p-3'>
                    <div className='flex items-center justify-between'>
                      <code className='text-xs font-mono truncate max-w-[220px]'>{r.trade_no}</code>
                      <TopupStatusBadge status={r.status} t={t} />
                    </div>
                    <div className='mt-2 grid grid-cols-3 gap-2 text-xs'>
                      <div>
                        <span className='text-muted-foreground'>{t('Quota')}: </span>
                        <span className='font-medium'>{formatQuota(r.amount)}</span>
                      </div>
                      <div>
                        <span className='text-muted-foreground'>{t('Paid')}: </span>
                        <span className='font-medium'>{formatNumber(r.money)}</span>
                      </div>
                      <div>
                        <span className='text-muted-foreground'>{t('Method')}: </span>
                        <span className='font-medium'>{r.payment_method || '-'}</span>
                      </div>
                      <div className='col-span-3'>
                        <span className='text-muted-foreground'>{t('Time')}: </span>
                        <span>{formatTimestampToDate(r.create_time)}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )
          ) : (
            logLoading ? (
              <div className='space-y-2'>
                {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className='h-14 w-full rounded-lg' />)}
              </div>
            ) : logs.length === 0 ? (
              <div className='text-muted-foreground py-12 text-center text-sm'>{t('No records')}</div>
            ) : (
              <div className='space-y-2 pr-2'>
                {logs.map((r) => (
                  <div key={r.id} className='rounded-lg border p-3'>
                    <div className='flex items-center justify-between'>
                      <span className='text-xs font-medium truncate max-w-[300px]'>{r.content}</span>
                      <LogTypeBadge type={r.type} t={t} />
                    </div>
                    <div className='mt-1.5 text-muted-foreground text-xs'>
                      {formatTimestampToDate(r.created_at)}
                    </div>
                  </div>
                ))}
              </div>
            )
          )}
        </ScrollArea>

        {/* Pagination */}
        {tab === 'topup' && topupTotal > PAGE_SIZE && (
          <Pagination page={topupPage} total={topupTotal} pageSize={PAGE_SIZE} onPageChange={setTopupPage} t={t} />
        )}
        {tab !== 'topup' && logTotal > PAGE_SIZE && (
          <Pagination page={logPage} total={logTotal} pageSize={PAGE_SIZE} onPageChange={setLogPage} t={t} />
        )}
      </DialogContent>
    </Dialog>
  )
}
