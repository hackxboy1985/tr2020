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
import { useState, useEffect, useCallback, useMemo } from 'react'
import { useQueryClient, useIsFetching } from '@tanstack/react-query'
import { useNavigate, getRouteApi } from '@tanstack/react-router'
import { type Table } from '@tanstack/react-table'
import { Eye, EyeOff, Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useIsAdmin } from '@/hooks/use-admin'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import type { ComboboxInputOption } from '@/components/ui/combobox-input'
import { SearchableFilterInput } from './searchable-filter-input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { LOG_TYPE_ALL_VALUE, LOG_TYPE_FILTERS } from '../constants'
import { buildSearchParams } from '../lib/filter'
import { getDefaultTimeRange } from '../lib/utils'
import type { CommonLogFilters } from '../types'
import { CommonLogsStats } from './common-logs-stats'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from './logs-filter-toolbar'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')
const logTypeValues = ['0', '1', '2', '3', '4', '5', '6'] as const

type LogTypeValue = (typeof logTypeValues)[number]

function isLogTypeValue(value: string): value is LogTypeValue {
  return (logTypeValues as readonly string[]).includes(value)
}

interface CommonLogsFilterBarProps<TData> {
  table: Table<TData>
}

export function CommonLogsFilterBar<TData>(
  props: CommonLogsFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const isAdmin = useIsAdmin()
  const { sensitiveVisible, setSensitiveVisible } = useUsageLogsContext()
  const fetchingLogs = useIsFetching({ queryKey: ['logs'] })

  const [filters, setFilters] = useState<CommonLogFilters>(() => {
    const { start, end } = getDefaultTimeRange()
    return { startTime: start, endTime: end }
  })
  const [logType, setLogType] = useState<LogTypeValue>(LOG_TYPE_ALL_VALUE)

  useEffect(() => {
    const { start, end } = getDefaultTimeRange()
    setFilters({
      startTime: searchParams.startTime
        ? new Date(searchParams.startTime)
        : start,
      endTime: searchParams.endTime ? new Date(searchParams.endTime) : end,
      channel: searchParams.channel || undefined,
      channelType: searchParams.channelType || undefined,
      model: searchParams.model || undefined,
      token: searchParams.token || undefined,
      group: searchParams.group || undefined,
      username: searchParams.username || undefined,
      requestId: searchParams.requestId || undefined,
      upstreamRequestId: searchParams.upstreamRequestId || undefined,
      taskId: searchParams.taskId || undefined,
    })

    const typeArr = searchParams.type
    const nextLogType =
      Array.isArray(typeArr) &&
      typeArr.length === 1 &&
      isLogTypeValue(typeArr[0])
        ? typeArr[0]
        : LOG_TYPE_ALL_VALUE
    setLogType(nextLogType)
  }, [
    searchParams.startTime,
    searchParams.endTime,
    searchParams.channel,
    searchParams.channelType,
    searchParams.model,
    searchParams.token,
    searchParams.group,
    searchParams.username,
    searchParams.requestId,
    searchParams.upstreamRequestId,
    searchParams.taskId,
    searchParams.type,
  ])

  const handleChange = useCallback(
    (field: keyof CommonLogFilters, value: Date | string | undefined) => {
      setFilters((prev) => ({ ...prev, [field]: value }))
    },
    []
  )

  const handleApply = useCallback(() => {
    const filterParams = buildSearchParams(filters, 'common')
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        ...filterParams,
        type: [logType],
        page: 1,
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
    queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
  }, [filters, logType, navigate, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    const resetFilters: CommonLogFilters = { startTime: start, endTime: end }
    setFilters(resetFilters)
    setLogType(LOG_TYPE_ALL_VALUE)

    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        page: 1,
        type: [LOG_TYPE_ALL_VALUE],
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
    queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const handleExport = useCallback(async () => {
    const filterParams = buildSearchParams(filters, 'common')
    const queryParams = new URLSearchParams()

    if (filterParams.startTime) queryParams.set('start_timestamp', String(filterParams.startTime))
    if (filterParams.endTime) queryParams.set('end_timestamp', String(filterParams.endTime))
    if (filterParams.model) queryParams.set('model', String(filterParams.model))
    if (filterParams.token) queryParams.set('token', String(filterParams.token))
    if (filterParams.channel) queryParams.set('channel', String(filterParams.channel))
    if (filterParams.channelType) queryParams.set('channelType', String(filterParams.channelType))
    if (filterParams.group) queryParams.set('group', String(filterParams.group))
    if (filterParams.username) queryParams.set('username', String(filterParams.username))
    if (filterParams.requestId) queryParams.set('request_id', String(filterParams.requestId))
    if (filterParams.upstreamRequestId) queryParams.set('upstream_request_id', String(filterParams.upstreamRequestId))
    if (filterParams.taskId) queryParams.set('task_id', String(filterParams.taskId))
    if (logType !== LOG_TYPE_ALL_VALUE) queryParams.set('type', logType)

    try {
      const response = await api.get(`/api/log/export?${queryParams.toString()}`, {
        responseType: 'blob',
      })

      const blob = new Blob([response.data], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
      })
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `logs_${new Date().toISOString().replace(/[:.]/g, '-')}.xlsx`
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } catch (error: any) {
      const message = error.response?.data?.message || error.message || 'Export failed'
      alert(message)
    }
  }, [filters, logType])

  const fetchTokenOptions = useCallback(
    async (keyword: string): Promise<ComboboxInputOption[]> => {
      const params = new URLSearchParams({ keyword, p: '1', size: '10' })
      const res = await fetch(`/api/token/search?${params}`)
      if (!res.ok) return []
      const data = await res.json()
      if (!data.success || !data.data?.items) return []
      return (data.data.items as Array<{ name: string }>).map((item) => ({
        value: item.name,
        label: item.name,
      }))
    },
    []
  )

  const fetchUserOptions = useCallback(
    async (keyword: string): Promise<ComboboxInputOption[]> => {
      const params = new URLSearchParams({ keyword })
      const res = await fetch(`/api/user/search?${params}`)
      if (!res.ok) return []
      const data = await res.json()
      if (!data.success || !data.data?.items) return []
      return (data.data.items as Array<{ username: string }>).map((item) => ({
        value: item.username,
        label: item.username,
      }))
    },
    []
  )

  const hasExpandedFilters =
    !!filters.token ||
    !!filters.username ||
    !!filters.channel ||
    !!filters.requestId ||
    !!filters.upstreamRequestId ||
    !!filters.taskId

  const hasTypeFilter = logType !== LOG_TYPE_ALL_VALUE
  const hasAdditionalFilters =
    !!filters.model || !!filters.group || hasTypeFilter || hasExpandedFilters

  const expandedFilterCount = [
    filters.token,
    isAdmin ? filters.username : undefined,
    isAdmin ? filters.channel : undefined,
    filters.requestId,
    filters.upstreamRequestId,
    filters.taskId,
  ].filter(Boolean).length
  const sensitiveType = sensitiveVisible ? 'text' : 'password'
  const logTypeItems = useMemo(
    () =>
      LOG_TYPE_FILTERS.map((type) => ({
        value: type.value,
        label: t(type.label),
      })),
    [t]
  )
  const logTypeLabel =
    logTypeItems.find((type) => type.value === logType)?.label ?? t('All Types')

  const statsBar = (
    <div className='flex flex-wrap items-center gap-2'>
      <CommonLogsStats />
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon'
              onClick={() => setSensitiveVisible(!sensitiveVisible)}
              aria-label={sensitiveVisible ? t('Hide') : t('Show')}
              className='text-muted-foreground hover:text-foreground size-7'
            />
          }
        >
          {sensitiveVisible ? <Eye /> : <EyeOff />}
        </TooltipTrigger>
        <TooltipContent>
          {sensitiveVisible ? t('Hide') : t('Show')}
        </TooltipContent>
      </Tooltip>
      {isAdmin && (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='ghost'
                size='icon'
                onClick={handleExport}
                aria-label={t('Export')}
                className='text-muted-foreground hover:text-foreground size-7'
              />
            }
          >
            <Download />
          </TooltipTrigger>
          <TooltipContent>{t('Export')}</TooltipContent>
        </Tooltip>
      )}
    </div>
  )

  const dateRangeFilter = (
    <LogsFilterField wide>
      <CompactDateTimeRangePicker
        start={filters.startTime}
        end={filters.endTime}
        onChange={({ start, end }) => {
          handleChange('startTime', start)
          handleChange('endTime', end)
        }}
      />
    </LogsFilterField>
  )
  const modelFilter = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('Model Name')}
        value={filters.model || ''}
        onChange={(e) => handleChange('model', e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const groupFilter = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('Group')}
        type={sensitiveType}
        value={filters.group || ''}
        onChange={(e) => handleChange('group', e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const typeFilter = (
    <LogsFilterField>
      <Select
        items={logTypeItems}
        value={logType}
        onValueChange={(value) => {
          setLogType(
            value !== null && isLogTypeValue(value) ? value : LOG_TYPE_ALL_VALUE
          )
        }}
      >
        <SelectTrigger>
          <SelectValue>{logTypeLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {LOG_TYPE_FILTERS.map((type) => (
              <SelectItem key={type.value} value={type.value}>
                {t(type.label)}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const advancedFilters = (
    <>
      <LogsFilterField>
        <SearchableFilterInput
          placeholder={t('Token Name')}
          value={filters.token || ''}
          onChange={(value) => handleChange('token', value)}
          fetchOptions={fetchTokenOptions}
          onEnter={handleApply}
        />
      </LogsFilterField>
      {isAdmin && (
        <LogsFilterField>
          <SearchableFilterInput
            placeholder={t('Username')}
            value={filters.username || ''}
            onChange={(value) => handleChange('username', value)}
            fetchOptions={fetchUserOptions}
            onEnter={handleApply}
          />
        </LogsFilterField>
      )}
      {isAdmin && (
        <LogsFilterField>
          <LogsFilterInput
            placeholder={t('Channel ID')}
            value={filters.channel || ''}
            onChange={(e) => handleChange('channel', e.target.value)}
            onKeyDown={handleKeyDown}
          />
        </LogsFilterField>
      )}
      {isAdmin && (
        <LogsFilterField>
          <Select
            value={filters.channelType?.toString() || ''}
            onValueChange={(value) =>
              handleChange('channelType', value ? Number(value) : undefined)
            }
          >
            <SelectTrigger>
              <SelectValue placeholder='渠道类型'>
                {filters.channelType === 1
                  ? '普通'
                  : filters.channelType === 2
                    ? '广告'
                    : undefined}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value="">{t('All')}</SelectItem>
                <SelectItem value="1">普通</SelectItem>
                <SelectItem value="2">广告</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </LogsFilterField>
      )}
      <LogsFilterField>
        <LogsFilterInput
          placeholder={t('Request ID')}
          value={filters.requestId || ''}
          onChange={(e) => handleChange('requestId', e.target.value)}
          onKeyDown={handleKeyDown}
        />
      </LogsFilterField>
      <LogsFilterField>
        <LogsFilterInput
          placeholder={t('Upstream Request ID')}
          value={filters.upstreamRequestId || ''}
          onChange={(e) => handleChange('upstreamRequestId', e.target.value)}
          onKeyDown={handleKeyDown}
        />
      </LogsFilterField>
      <LogsFilterField>
        <LogsFilterInput
          placeholder={t('Task ID')}
          value={filters.taskId || ''}
          onChange={(e) => handleChange('taskId', e.target.value)}
          onKeyDown={handleKeyDown}
        />
      </LogsFilterField>
    </>
  )

  return (
    <LogsFilterToolbar
      table={props.table}
      stats={statsBar}
      primaryFilters={
        <>
          {dateRangeFilter}
          {modelFilter}
          {groupFilter}
          {typeFilter}
        </>
      }
      advancedFilters={advancedFilters}
      mobilePinnedFilters={dateRangeFilter}
      mobileFilters={
        <>
          {modelFilter}
          {groupFilter}
          {typeFilter}
          {advancedFilters}
        </>
      }
      mobileFilterCount={
        [filters.model, filters.group, hasTypeFilter].filter(Boolean).length +
        expandedFilterCount
      }
      hasAdvancedActiveFilters={hasExpandedFilters}
      advancedFilterCount={expandedFilterCount}
      hasActiveFilters={hasAdditionalFilters}
      onSearch={handleApply}
      searchLoading={fetchingLogs > 0}
      onReset={handleReset}
    />
  )
}
