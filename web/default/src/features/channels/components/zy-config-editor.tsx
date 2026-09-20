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
import { useMemo, useState } from 'react'
import { Code, Table } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

/**
 * Zy channel `other` config shape (all fields optional).
 * Unknown keys are preserved so that JSON-only settings
 * (e.g. zy_url_proxy_mappings) survive visual-mode edits.
 */
type ZyConfig = Record<string, unknown> & {
  zy_url_ttl_hours?: number
  zy_url_proxy_base_url?: string
}

type ParsedConfig = {
  ok: boolean
  ttlHours: number
  proxyBaseUrl: string
  raw: ZyConfig
}

type ZyConfigEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

function parseConfig(value: string): ParsedConfig {
  const fallback: ParsedConfig = {
    ok: true,
    ttlHours: 0,
    proxyBaseUrl: '',
    raw: {},
  }
  if (!value.trim()) return fallback

  try {
    const parsed: unknown = JSON.parse(value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return { ...fallback, ok: false }
    }
    const raw = parsed as ZyConfig
    return {
      ok: true,
      ttlHours:
        typeof raw.zy_url_ttl_hours === 'number' ? raw.zy_url_ttl_hours : 0,
      proxyBaseUrl:
        typeof raw.zy_url_proxy_base_url === 'string'
          ? raw.zy_url_proxy_base_url
          : '',
      raw,
    }
  } catch {
    return { ...fallback, ok: false }
  }
}

export function ZyConfigEditor(props: ZyConfigEditorProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')

  // Derived from props: the parent form owns the value, so no mirrored state
  // and no synchronising effect is needed.
  const parsed = useMemo(() => parseConfig(props.value), [props.value])

  const ttlHours = parsed.ttlHours
  const proxyBaseUrl = parsed.proxyBaseUrl

  // Rebuild the JSON string, merging the visual fields into the existing
  // object so unrelated keys are never dropped.
  const buildJson = (ttl: number, proxyUrl: string): string => {
    const config: ZyConfig = { ...parsed.raw }
    if (ttl > 0) {
      config.zy_url_ttl_hours = ttl
    } else {
      delete config.zy_url_ttl_hours
    }
    const trimmed = proxyUrl.trim()
    if (trimmed) {
      config.zy_url_proxy_base_url = trimmed
    } else {
      delete config.zy_url_proxy_base_url
    }
    if (Object.keys(config).length === 0) {
      return ''
    }
    return JSON.stringify(config, null, 2)
  }

  const handleTtlChange = (raw: string) => {
    const next = Number(raw)
    const ttl = Number.isFinite(next) && next > 0 ? next : 0
    props.onChange(buildJson(ttl, proxyBaseUrl))
  }

  const handleProxyBaseUrlChange = (value: string) => {
    props.onChange(buildJson(ttlHours, value))
  }

  const handleModeChange = (nextMode: string) => {
    if (nextMode !== 'visual' && nextMode !== 'json') return
    if (nextMode === 'json') {
      // Normalise to the canonical JSON built from the visual fields.
      props.onChange(buildJson(ttlHours, proxyBaseUrl))
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

        {!parsed.ok && (
          <Alert variant='destructive'>
            <AlertDescription>{t('Invalid JSON format')}</AlertDescription>
          </Alert>
        )}

        <TabsContent value='visual' className='space-y-4'>
          {/* URL TTL */}
          <div className='space-y-1'>
            <div className='text-sm font-medium'>{t('URL TTL (hours)')}</div>
            <Input
              type='number'
              min={0}
              value={ttlHours}
              onChange={(e) => handleTtlChange(e.target.value)}
              placeholder='0'
              disabled={props.disabled}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Upstream image URL expiry in hours. Leave empty or 0 to never expire, returning the original upstream URL. Requests after expiry return 410 Gone.'
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
                'Optional. Replaces upstream image URL scheme and host for Zy result URLs. If the proxy URL includes a path, it is prepended to the upstream path. Query is kept unchanged.'
              )}
            </p>
          </div>
        </TabsContent>

        <TabsContent value='json'>
          <Textarea
            value={props.value}
            onChange={(e) => props.onChange(e.target.value)}
            placeholder={
              '{\n  "zy_url_ttl_hours": 5,\n  "zy_url_proxy_base_url": "https://img.example.com/oss"\n}'
            }
            disabled={props.disabled}
            rows={6}
            className={cn(
              'font-mono text-sm',
              !parsed.ok && 'border-destructive'
            )}
            aria-invalid={!parsed.ok}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}
