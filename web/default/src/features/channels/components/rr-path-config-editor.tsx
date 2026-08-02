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
import { useEffect, useRef, useState } from 'react'
import { Code, Plus, Table, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

const RR_MODELS = ['rhart-image-g-2-official']

const CONDITION_OPTIONS = [
  { value: '', label: 'Always' },
  { value: ':image', label: 'Has images' },
] as const

const KNOWN_CONDITION_SUFFIXES = CONDITION_OPTIONS.filter((o) => o.value !== '').map((o) => o.value)

function parseEndpointKey(key: string): { model: string; condition: string } {
  for (const suffix of KNOWN_CONDITION_SUFFIXES) {
    if (key.endsWith(suffix)) {
      return { model: key.slice(0, -suffix.length), condition: suffix }
    }
  }
  return { model: key, condition: '' }
}

type EndpointRow = {
  id: string
  model: string
  condition: string
  path: string
}

type RRPathConfig = {
  rr_endpoints?: Record<string, string>
  rr_url_ttl_hours?: number
}

type RRPathConfigEditorProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

export function RRPathConfigEditor(props: RRPathConfigEditorProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<EndpointRow[]>([])
  const [ttlHours, setTtlHours] = useState<number>(24)
  const [jsonValue, setJsonValue] = useState(props.value)
  const [jsonError, setJsonError] = useState<string | null>(null)
  const nextRowIdRef = useRef(0)

  const createRowId = () => {
    nextRowIdRef.current += 1
    return `endpoint-${nextRowIdRef.current}`
  }

  const buildJson = (
    endpointRows: EndpointRow[],
    ttl: number
  ): string => {
    const config: RRPathConfig = {}
    const endpoints: Record<string, string> = {}
    for (const row of endpointRows) {
      if (row.model.trim() && row.path.trim()) {
        endpoints[row.model.trim() + row.condition] = row.path.trim()
      }
    }
    if (Object.keys(endpoints).length > 0) {
      config.rr_endpoints = endpoints
    }
    if (ttl > 0) {
      config.rr_url_ttl_hours = ttl
    }
    if (Object.keys(config).length === 0) {
      return ''
    }
    return JSON.stringify(config, null, 2)
  }

  const parseJson = (json: string): boolean => {
    try {
      if (!json.trim()) {
        setRows([])
        setTtlHours(24)
        setJsonError(null)
        return true
      }
      const parsed: RRPathConfig = JSON.parse(json)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        setJsonError(t('Must be a valid JSON object'))
        return false
      }
      const endpoints = parsed.rr_endpoints ?? {}
      setRows(
        Object.entries(endpoints).map(([key, path]) => {
          const { model, condition } = parseEndpointKey(key)
          return {
            id: createRowId(),
            model,
            condition,
            path,
          }
        })
      )
      setTtlHours(parsed.rr_url_ttl_hours ?? 24)
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

  const syncVisual = (endpointRows: EndpointRow[], ttl: number) => {
    const json = buildJson(endpointRows, ttl)
    setJsonValue(json)
    setJsonError(null)
    props.onChange(json)
  }

  const handleTtlChange = (v: number) => {
    const ttl = v > 0 ? v : 1
    setTtlHours(ttl)
    syncVisual(rows, ttl)
  }

  const handleAddRow = () => {
    const newRows = [...rows, { id: createRowId(), model: '', condition: '', path: '' }]
    setRows(newRows)
    syncVisual(newRows, ttlHours)
  }

  const handleDeleteRow = (id: string) => {
    const newRows = rows.filter((r) => r.id !== id)
    setRows(newRows)
    syncVisual(newRows, ttlHours)
  }

  const handleRowChange = (
    id: string,
    field: 'model' | 'condition' | 'path',
    value: string
  ) => {
    const newRows = rows.map((r) =>
      r.id === id ? { ...r, [field]: value } : r
    )
    setRows(newRows)
    syncVisual(newRows, ttlHours)
  }

  const handleJsonChange = (newJson: string) => {
    setJsonValue(newJson)
    props.onChange(newJson)
    parseJson(newJson)
  }

  const handleModeChange = (nextMode: string) => {
    if (nextMode !== 'visual' && nextMode !== 'json') return
    if (nextMode === 'json') {
      const json = buildJson(rows, ttlHours)
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

          {/* Endpoint Overrides */}
          <div className='space-y-2'>
            <div className='text-sm font-medium'>{t('Endpoint Overrides')}</div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Override upstream paths per model. Default: /openapi/v2/{model}/text-to-image'
              )}
            </p>
            {rows.length > 0 ? (
              <div className='space-y-2'>
                <div className='grid grid-cols-[1fr_130px_1fr_auto] gap-2 text-sm font-medium'>
                  <div>{t('Model')}</div>
                  <div>{t('Condition')}</div>
                  <div>{t('Path')}</div>
                  <div className='w-10' />
                </div>
                {rows.map((row) => (
                  <div
                    key={row.id}
                    className='grid grid-cols-[1fr_130px_1fr_auto] gap-2'
                  >
                    <Input
                      value={row.model}
                      onChange={(e) =>
                        handleRowChange(row.id, 'model', e.target.value)
                      }
                      placeholder='rhart-image-g-2-official'
                      disabled={props.disabled}
                      list='rr-model-list'
                    />
                    <NativeSelect
                      value={row.condition}
                      onChange={(e) => handleRowChange(row.id, 'condition', e.target.value)}
                      disabled={props.disabled}
                    >
                      {CONDITION_OPTIONS.map((opt) => (
                        <NativeSelectOption key={opt.value} value={opt.value}>
                          {t(opt.label)}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                    <Input
                      value={row.path}
                      onChange={(e) =>
                        handleRowChange(row.id, 'path', e.target.value)
                      }
                      placeholder='/openapi/v2/rhart-image-g-2-official/text-to-image'
                      disabled={props.disabled}
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      onClick={() => handleDeleteRow(row.id)}
                      disabled={props.disabled}
                      className='h-10 w-10'
                      aria-label={t('Delete')}
                    >
                      <Trash2 className='h-4 w-4' aria-hidden='true' />
                    </Button>
                  </div>
                ))}
              </div>
            ) : (
              <div className='text-muted-foreground flex h-20 items-center justify-center rounded-md border border-dashed text-sm'>
                {t('No endpoint overrides. Click "Add Override" to add one.')}
              </div>
            )}
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleAddRow}
              disabled={props.disabled}
              className='w-full'
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add Override')}
            </Button>
          </div>
        </TabsContent>

        <TabsContent value='json'>
          <Textarea
            value={jsonValue}
            onChange={(e) => handleJsonChange(e.target.value)}
            placeholder={
              '{\n  "rr_endpoints": {\n    "rhart-image-g-2-official": "/openapi/v2/rhart-image-g-2-official/text-to-image"\n  },\n  "rr_url_ttl_hours": 24\n}'
            }
            disabled={props.disabled}
            rows={8}
            className={cn(
              'font-mono text-sm',
              jsonError && 'border-destructive'
            )}
            aria-invalid={Boolean(jsonError)}
          />
        </TabsContent>
      </Tabs>

      <datalist id='rr-model-list'>
        {RR_MODELS.map((m) => (
          <option key={m} value={m} />
        ))}
      </datalist>
    </div>
  )
}
