import { useEffect, useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, AlertTriangle, CheckCircle, XCircle, TrendingDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

type HeartbeatResult = {
  success: boolean
  response_time: number
}

type HeartbeatRecord = {
  tested_at: number
  color: 'green' | 'yellow' | 'orange' | 'red'
  results: Record<number, HeartbeatResult>
}

type ChannelInfo = {
  id?: number
  name?: string
  display_name?: string
  priority: number
  response_time: number
  status: number
}

type DisabledChannelInfo = {
  id?: number
  name?: string
  display_name?: string
  priority: number
  status: number
}

type GroupMonitorResult = {
  group: string
  status: 'up' | 'degraded' | 'down'
  uptime_24h: number
  avg_latency: number
  last_tested_at: number
  top_channels: ChannelInfo[]
  disabled_higher_priority_channels: DisabledChannelInfo[]
  heartbeats: HeartbeatRecord[]
}

const HEARTBEAT_COLORS: Record<string, string> = {
  green: 'bg-green-500',
  yellow: 'bg-amber-400',
  orange: 'bg-orange-500',
  red: 'bg-red-500',
}

const HEARTBEAT_TOOLTIP: Record<string, string> = {
  green: '正常 - 最高优先级渠道可用',
  yellow: '降级 - 次优先级渠道顶替',
  orange: '严重降级 - 仅低优先级可用',
  red: '故障 - 所有渠道不可用',
}

function formatTimeAgo(timestamp: number): string {
  const diff = Math.floor(Date.now() / 1000) - timestamp
  if (diff < 60) return `${diff}秒前`
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${Math.floor(diff / 86400)}天前`
}

function StatusBadge({ status }: { status: GroupMonitorResult['status'] }) {
  const { t } = useTranslation()
  if (status === 'up') {
    return (
      <Badge variant='outline' className='text-green-600 border-green-600 gap-1'>
        <CheckCircle className='h-3 w-3' />
        {t('Running normally')}
      </Badge>
    )
  }
  if (status === 'degraded') {
    return (
      <Badge variant='outline' className='text-amber-600 border-amber-600 gap-1'>
        <TrendingDown className='h-3 w-3' />
        {t('Degraded')}
      </Badge>
    )
  }
  return (
    <Badge variant='outline' className='text-red-600 border-red-600 gap-1'>
      <XCircle className='h-3 w-3' />
      {t('Down')}
    </Badge>
  )
}

function HeartbeatGrid({ heartbeats }: { heartbeats: HeartbeatRecord[] }) {
  // 按时间升序展示（最左边是最早的，最右边是最新的）
  const sorted = [...heartbeats].sort((a, b) => a.tested_at - b.tested_at)

  return (
    <TooltipProvider>
      <div className='flex gap-0.5 items-center'>
        {sorted.map((hb, idx) => (
          <Tooltip key={idx}>
            <TooltipTrigger asChild>
              <div
                className={`w-2 h-6 rounded-sm cursor-pointer ${HEARTBEAT_COLORS[hb.color] ?? 'bg-gray-300'}`}
              />
            </TooltipTrigger>
            <TooltipContent side='top' className='text-xs'>
              <p>{HEARTBEAT_TOOLTIP[hb.color]}</p>
              <p className='text-muted-foreground'>
                {formatTimeAgo(hb.tested_at)}
              </p>
            </TooltipContent>
          </Tooltip>
        ))}
        {/* 填充空格子（如果历史不足 60 条） */}
        {Array.from({ length: Math.max(0, 60 - sorted.length) }).map(
          (_, idx) => (
            <div key={`empty-${idx}`} className='w-2 h-6 rounded-sm bg-gray-200' />
          )
        )}
      </div>
    </TooltipProvider>
  )
}

function GroupCard({ group }: { group: GroupMonitorResult }) {
  const { t } = useTranslation()
  const channelName = (ch: ChannelInfo | DisabledChannelInfo) =>
    ch.name ?? ch.display_name ?? `Priority ${ch.priority}`

  return (
    <Card>
      <CardHeader className='pb-2'>
        <div className='flex items-center justify-between'>
          <CardTitle className='text-base font-semibold'>{group.group}</CardTitle>
          <StatusBadge status={group.status} />
        </div>

        <div className='flex items-center gap-4 text-sm text-muted-foreground mt-1'>
          <span>
            {t('Availability')}: {(group.uptime_24h * 100).toFixed(2)}%
          </span>
          <span>
            {t('Latency')}: {group.avg_latency}ms
          </span>
          {group.last_tested_at > 0 && (
            <span>
              {t('Last tested')}: {formatTimeAgo(group.last_tested_at)}
            </span>
          )}
        </div>
      </CardHeader>

      <CardContent className='space-y-3'>
        {/* 降级告警 */}
        {group.disabled_higher_priority_channels?.length > 0 && (
          <div className='flex items-start gap-2 text-sm text-amber-700 bg-amber-50 rounded-md px-3 py-2'>
            <AlertTriangle className='h-4 w-4 mt-0.5 flex-shrink-0' />
            <div>
              {group.disabled_higher_priority_channels.map((ch, i) => (
                <span key={i}>
                  {channelName(ch)}（priority={ch.priority}）
                  {i < group.disabled_higher_priority_channels.length - 1 ? '、' : ''}
                </span>
              ))}
              <span className='ml-1'>{t('temporarily unavailable')}</span>
            </div>
          </div>
        )}

        {/* 前3渠道信息 */}
        {group.top_channels?.length > 0 && (
          <div className='text-xs text-muted-foreground'>
            {t('Top channels')}:{' '}
            {group.top_channels.map((ch, i) => (
              <span key={i}>
                {channelName(ch)}（priority={ch.priority}
                {ch.response_time > 0 ? `, ${ch.response_time}ms` : ''}）
                {i < group.top_channels.length - 1 ? ' > ' : ''}
              </span>
            ))}
          </div>
        )}

        {/* 心跳格 */}
        <div>
          <div className='flex justify-between text-xs text-muted-foreground mb-1'>
            <span>{t('Past')}</span>
            <span>{t('Latest')} {t('Last 60 tests')}</span>
          </div>
          <HeartbeatGrid heartbeats={group.heartbeats ?? []} />
        </div>

        {/* 图例 */}
        <div className='flex gap-3 text-xs text-muted-foreground'>
          <span className='flex items-center gap-1'>
            <span className='inline-block w-2 h-2 rounded-sm bg-green-500' />
            {t('Normal')}
          </span>
          {group.top_channels?.length >= 2 && (
            <span className='flex items-center gap-1'>
              <span className='inline-block w-2 h-2 rounded-sm bg-amber-400' />
              {t('Degraded')}
            </span>
          )}
          {group.top_channels?.length >= 3 && (
            <span className='flex items-center gap-1'>
              <span className='inline-block w-2 h-2 rounded-sm bg-orange-500' />
              {t('Severely degraded')}
            </span>
          )}
          <span className='flex items-center gap-1'>
            <span className='inline-block w-2 h-2 rounded-sm bg-red-500' />
            {t('Down')}
          </span>
        </div>
      </CardContent>
    </Card>
  )
}

export function GroupMonitorPanel() {
  const { t } = useTranslation()
  const [groups, setGroups] = useState<GroupMonitorResult[]>([])
  const [loading, setLoading] = useState(false)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/user/group-monitor', {
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
      })
      const data = await res.json()
      if (data.success) {
        setGroups(data.data ?? [])
        setLastUpdated(new Date())
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const allNormal = groups.length > 0 && groups.every((g) => g.status === 'up')
  const degradedCount = groups.filter((g) => g.status === 'degraded').length
  const downCount = groups.filter((g) => g.status === 'down').length

  return (
    <div className='space-y-4'>
      {/* 顶部汇总 */}
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-3'>
          <div
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium ${
              allNormal
                ? 'bg-green-50 text-green-700'
                : downCount > 0
                  ? 'bg-red-50 text-red-700'
                  : 'bg-amber-50 text-amber-700'
            }`}
          >
            {allNormal ? (
              <>
                <CheckCircle className='h-4 w-4' />
                {t('All groups running normally')}
              </>
            ) : (
              <>
                <AlertTriangle className='h-4 w-4' />
                {downCount > 0
                  ? `${downCount} ${t('groups down')}`
                  : `${degradedCount} ${t('groups degraded')}`}
              </>
            )}
          </div>

          <span className='text-sm text-muted-foreground'>
            {groups.length} {t('groups')}
          </span>

          {lastUpdated && (
            <span className='text-xs text-muted-foreground'>
              {t('Last updated')}: {lastUpdated.toLocaleTimeString()}
            </span>
          )}
        </div>

        <Button
          variant='outline'
          size='sm'
          onClick={fetchData}
          disabled={loading}
          className='gap-1'
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          {t('Refresh')}
        </Button>
      </div>

      {/* 分组卡片网格 */}
      {loading && groups.length === 0 ? (
        <div className='text-center py-12 text-muted-foreground'>
          {t('Loading...')}
        </div>
      ) : groups.length === 0 ? (
        <div className='text-center py-12 text-muted-foreground'>
          <p>{t('No groups configured for monitoring')}</p>
          <p className='text-sm mt-1'>
            {t(
              'Go to System Settings → Operations → Monitoring to select groups to display'
            )}
          </p>
        </div>
      ) : (
        <div className='grid gap-4 md:grid-cols-2'>
          {groups.map((group) => (
            <GroupCard key={group.group} group={group} />
          ))}
        </div>
      )}
    </div>
  )
}
