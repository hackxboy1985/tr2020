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
import { useEffect, useState } from 'react'
import { Code, Table } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

type TdConfig = {
  td_url_ttl_hours?: number
  td_url_proxy_base_url?: string
}

type TdConfigEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

export function TdConfigEditor(props: TdConfigEditorProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [ttlHours, setTtlHours] = useState<number>(24)
  const [proxyBaseUrl, setProxyBaseUrl] = useState('')
  const [jsonValue, setJsonValue] = useState(props.value)
  const [jsonError, setJsonError] = useState<string | null>(null)

  const buildJson = (ttl: number, proxyUrl: string): string => {
    const config: TdConfig = {}
    if (ttl > 0) {
      config.td_url_ttl_hours = ttl
    }
    if (proxyUrl.trim()) {
      config.td_url_proxy_base_url = proxyUrl.trim()
    }
    if (Object.keys(config).length === 0) {
      return ''
    }
    return JSON.stringify(config, null, 2)
  }

  const parseJson = (json: string): boolean => {
    try {
      if (!json.trim()) {
        setTtlHours(24)
        setProxyBaseUrl('')
        setJsonError(null)
        return true
      }
      const parsed: TdConfig = JSON.parse(json)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        setJsonError(t('Must be a valid JSON object'))
        return false
      }
      setTtlHours(parsed.td_url_ttl_hours ?? 24)
      setProxyBaseUrl(parsed.td_url_proxy_base_url ?? '')
      setJsonError(null)
      return true
    } catch (_e) {
      setJsonError(t('Invalid JSON format'))
      return false
    }
  }

  useEffect(() => {
    setJsonValue(props.value)
    parseJson(props.value)
  }, [props.value])

  const syncVisual = (ttl: number, proxyUrl: string) => {
    const json = buildJson(ttl, proxyUrl)
    setJsonValue(json)
    setJsonError(null)
    props.onChange(json)
  }

  const handleTtlChange = (v: number) => {
    const ttl = v > 0 ? v : 1
    setTtlHours(ttl)
    syncVisual(ttl, proxyBaseUrl)
  }

  const handleProxyBaseUrlChange = (value: string) => {
    setProxyBaseUrl(value)
    syncVisual(ttlHours, value)
  }

  const handleJsonChange = (newJson: string) => {
    setJsonValue(newJson)
    props.onChange(newJson)
    parseJson(newJson)
  }

  const handleModeChange = (nextMode: string) => {
    if (nextMode !== 'visual' && nextMode !== 'json') return
    if (nextMode === 'json') {
      const json = buildJson(ttlHours, proxyBaseUrl)
      setJsonValue(json)
      props.onChange(json)
    } else {
      parseJson(jsonValue)
    }
    setMode(nextMode)
  }

  return (
    <div className='space-y-2'>
      <Tabs value={mode} onValueChange={handleModeChange} className='space-y-2'>
        <TabsList>
          <TabsTrigger value='visual'>
            <Table className='h-4 w-4' aria-hidden='true' />
            {t('Visual')}
          </TabsTrigger>
          <TabsTrigger value='json'>
            <Code className='h-4 w-4' aria-hidden='true' />
            {t('JSON')}
          </TabsTrigger>
        </TabsList>

        {jsonError && (
          <Alert variant='destructive'>
            <AlertDescription>{jsonError}</AlertDescription>
          </Alert>
        )}

        <TabsContent value='visual' className='space-y-4'>
          {/* URL TTL */}
          <div className='space-y-1'>
            <div className='text-sm font-medium'>{t('URL TTL (hours)')}</div>
            <Input
              type='number'
              min={1}
              value={ttlHours}
              onChange={(e) => handleTtlChange(Number(e.target.value))}
              placeholder='24'
              disabled={props.disabled}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Upstream image URL expiry in hours. Requests after expiry return 410 Gone. Default: 24.'
              )}
            </p>
          </div>

          {/* URL Proxy Base URL */}
          <div className='space-y-1'>
            <div className='text-sm font-medium'>{t('URL Proxy Base URL')}</div>
            <Input
              value={proxyBaseUrl}
              onChange={(e) => handleProxyBaseUrlChange(e.target.value)}
              placeholder='https://img.example.com/oss'
              disabled={props.disabled}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Optional. Replaces upstream image URL scheme and host for Td result URLs. If the proxy URL includes a path, it is prepended to the upstream path. Query is kept unchanged.'
              )}
            </p>
          </div>
        </TabsContent>

        <TabsContent value='json'>
          <Textarea
            value={jsonValue}
            onChange={(e) => handleJsonChange(e.target.value)}
            placeholder={
              '{\n  "td_url_ttl_hours": 24,\n  "td_url_proxy_base_url": "https://img.example.com/oss"\n}'
            }
            disabled={props.disabled}
            rows={6}
            className={cn(
              'font-mono text-sm',
              jsonError && 'border-destructive'
            )}
            aria-invalid={Boolean(jsonError)}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}
